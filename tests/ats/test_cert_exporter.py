import logging
import requests
import time
import tempfile
import subprocess
from functools import partial
from pathlib import Path
from json import dumps
from typing import Dict, List
from datetime import datetime, timedelta

import pykube
import pytest
from pytest_helm_charts.fixtures import Cluster
from pytest_helm_charts.k8s.deployment import wait_for_deployments_to_run
from pytest_helm_charts.k8s.daemon_set import wait_for_daemon_sets_to_run
from pytest_helm_charts.giantswarm_app_platform.app import (
    AppFactoryFunc,
    ConfiguredApp,
)

from cert_util import cert_gen


logger = logging.getLogger(__name__)

app_name = "cert-exporter"
namespace_name = "kube-system"
cert_manager_app_chart_version = "2.9.0"
# change this if your want to change the kind config
daemonset_port = 30017
deployment_port = 30018
host_ports = [
    daemonset_port,
    deployment_port,
]

timeout: int = 360


def prepare_services(kube_cluster: Cluster) -> None:
    kubectl = partial(kube_cluster.kubectl, namespace=namespace_name, output_format="")

    # patch services to be NodePort services
    # otherwise tests won't work
    ce_patch = dumps(
        {
            "spec": {
                "type": "NodePort",
                "ports": [{"name": "cert-exporter", "port": 9005, "nodePort": 30007}],
            }
        }
    )
    kubectl("patch service cert-exporter-daemonset", patch=ce_patch)
    logger.info("Patched cert-exporter-daemonset service")

    sce_patch = dumps(
        {
            "spec": {
                "type": "NodePort",
                "ports": [
                    {"name": "cert-exporter", "port": 9005, "nodePort": 30008}
                ],
            }
        }
    )
    kubectl("patch service cert-exporter-deployment", patch=sce_patch)
    logger.info("Patched cert-exporter-deployment service")


@pytest.mark.smoke
@pytest.mark.upgrade
def test_api_working(kube_cluster: Cluster) -> None:
    """Very minimalistic example of using the [kube_cluster](pytest_helm_charts.fixtures.kube_cluster)
    fixture to get an instance of [Cluster](pytest_helm_charts.clusters.Cluster) under test
    and access its [kube_client](pytest_helm_charts.clusters.Cluster.kube_client) property
    to get access to Kubernetes API of cluster under test.
    Please refer to [pykube](https://pykube.readthedocs.io/en/latest/api/pykube.html) to get docs
    for [HTTPClient](https://pykube.readthedocs.io/en/latest/api/pykube.html#pykube.http.HTTPClient).
    """
    assert kube_cluster.kube_client is not None
    assert len(pykube.Node.objects(kube_cluster.kube_client)) >= 1


@pytest.mark.smoke
@pytest.mark.upgrade
def test_cluster_info(
    kube_cluster: Cluster, cluster_type: str, test_extra_info: Dict[str, str]
) -> None:
    """Example shows how you can access additional information about the cluster the tests are running on"""
    logger.info(f"Running on cluster type {cluster_type}")
    key = "external_cluster_type"
    if key in test_extra_info:
        logger.info(f"{key} is {test_extra_info[key]}")
    assert kube_cluster.kube_client is not None
    assert cluster_type != ""


@pytest.fixture(scope="module")
def certexporter_deployment(kube_cluster: Cluster) -> List[pykube.Deployment]:
    deployments = wait_for_deployments_to_run(
        kube_cluster.kube_client,
        ["cert-exporter-deployment"],
        namespace_name,
        timeout,
    )
    return deployments


@pytest.fixture(scope="module")
def cert_manager_app_cr(app_factory: AppFactoryFunc) -> ConfiguredApp:
    res = app_factory(
        app_name="cert-manager-app",  # app_name
        app_version=cert_manager_app_chart_version,  # app_version
        catalog_name="giantswarm-stable",  # catalog_name
        catalog_namespace="giantswarm",  # catalog_namespace
        catalog_url="https://giantswarm.github.io/giantswarm-catalog/",  # catalog_url
        namespace="cert-manager-app",
        deployment_namespace="cert-manager-app",
        timeout_sec=timeout,
    )
    return res


@pytest.fixture(scope="module")
def certexporter_daemonset(kube_cluster: Cluster) -> List[pykube.DaemonSet]:
    daemonsets = wait_for_daemon_sets_to_run(
        kube_cluster.kube_client,
        ["cert-exporter-daemonset"],
        namespace_name,
        timeout,
    )
    return daemonsets


def retrieve_metrics(port: int) -> List[str]:
    r = requests.get(f"http://127.0.0.1:{port}/metrics")

    assert r.status_code == 200

    raw_metrics = r.text
    metrics_lines = [
        line for line in raw_metrics.splitlines() if not line.startswith("#")
    ]

    assert len(metrics_lines) != 0

    return metrics_lines


def check_expiry(ts):
    def check(metric):
        val = metric.split(" ")[1]
        return abs(int(float(val)) - ts) < 5

    return check


def assert_metric(metrics: List[str], metric: str, validator=None) -> None:
    if validator is None:
        found = [m for m in metrics if m.startswith(metric)]
    else:
        found = [m for m in metrics if m.startswith(metric) and validator(m)]
    assert len(found) == 1, f"Expected 1 metric for '{metric}', but found {len(found)}.\nMetrics: {metrics}"


@pytest.mark.smoke
@pytest.mark.upgrade
def test_pods_available(
    kube_cluster: Cluster,
    certexporter_deployment: List[pykube.Deployment],
    certexporter_daemonset: List[pykube.DaemonSet],
):
    for s in certexporter_deployment:
        logger.info(
            f"Deployment '{s.name}' has {s.obj['status']['readyReplicas']} readyReplicas"
        )
        assert int(s.obj["status"]["readyReplicas"]) > 0
    for ds in certexporter_daemonset:
        logger.info(
            f"DaemonSet '{ds.name}' has status numberReady = {ds.obj['status']['numberReady']}"
        )
        assert int(ds.obj["status"]["numberReady"]) > 0


@pytest.mark.smoke
@pytest.mark.upgrade
def test_exporters_reachable(
    kube_cluster: Cluster,
    certexporter_deployment: List[pykube.Deployment],
    certexporter_daemonset: List[pykube.DaemonSet],
):
    # patch services to be type: NodePort
    prepare_services(kube_cluster)

    # let the dust settle
    time.sleep(10)

    # check if exporters respond to anything
    for port in host_ports:
        retrieve_metrics(port)


@pytest.mark.functional
@pytest.mark.upgrade
def test_file_certificate_metrics(
    kube_cluster: Cluster,
    certexporter_deployment: List[pykube.Deployment],
    certexporter_daemonset: List[pykube.DaemonSet],
):
    # patch services to be type: NodePort
    prepare_services(kube_cluster)

    metric_name = "cert_exporter_not_after"

    cert_name = "filesystem-cert"

    expires_in_secs = 3600
    expected_expiry = int(
        (datetime.now() + timedelta(seconds=expires_in_secs)).strftime("%s")
    )

    with tempfile.TemporaryDirectory() as tmpdirname:
        cert = cert_gen(name=cert_name, not_after=expires_in_secs)[0]
        cert_path = f"{tmpdirname}/{cert_name}.crt"

        with open(cert_path, "wt") as f:
            f.write(cert)

        # kind-control-plane is the container name
        subprocess.run(
            ["docker", "cp", cert_path, "kind-control-plane:/certs"],
            check=True,
        )

    # let the dust settle
    time.sleep(5)

    # request from daemonset port
    ds_metrics = retrieve_metrics(daemonset_port)
    assert_metric(
        ds_metrics,
        f'{metric_name}{{path="/certs/{cert_name}.crt"}}',
        check_expiry(expected_expiry),
    )

    # request from deployment port
    deploy_metrics = retrieve_metrics(deployment_port)
    assert len([m for m in deploy_metrics if m.startswith(metric_name)]) == 0

    # cleanup
    subprocess.run(
        ["docker", "exec", "kind-control-plane", "bash", "-c", "rm -rf /certs/*"],
        check=True,
    )


@pytest.mark.functional
@pytest.mark.upgrade
def test_secret_metrics(
    kube_cluster: Cluster,
    certexporter_deployment: List[pykube.Deployment],
):
    # patch services to be type: NodePort
    prepare_services(kube_cluster)

    metric_name = "cert_exporter_secret_not_after"

    cert_name = "secret-cert"

    expires_in_secs = 3600
    expected_expiry = int(
        (datetime.now() + timedelta(seconds=expires_in_secs)).strftime("%s")
    )

    # Create a kubernetes.io/tls secret
    with tempfile.TemporaryDirectory() as tmpdirname:
        (cert, key) = cert_gen(name=cert_name, not_after=expires_in_secs)
        cert_path = f"{tmpdirname}/{cert_name}.crt"
        key_path = f"{tmpdirname}/{cert_name}.key"

        with open(cert_path, "wt") as f:
            f.write(cert)

        with open(key_path, "wt") as f:
            f.write(key)

        kube_cluster.kubectl(
            f"create secret tls {cert_name}",
            output_format="",
            cert=cert_path,
            key=key_path,
        )

    # let the dust settle
    time.sleep(5)

    # request from deployment port
    deploy_metrics = retrieve_metrics(deployment_port)
    assert_metric(
        deploy_metrics,
        f'{metric_name}{{certificatename="",name="{cert_name}",namespace="default",secretkey="tls.crt"}}',
        check_expiry(expected_expiry),
    )

    # request from daemonset port
    ds_metrics = retrieve_metrics(daemonset_port)
    assert len([m for m in ds_metrics if m.startswith(metric_name)]) == 0

    # cleanup
    kube_cluster.kubectl(
        f"delete secret {cert_name}",
        output_format="",
    )


@pytest.mark.functional
# @pytest.mark.upgrade
def test_certificate_cr_metrics(
    request,
    kube_cluster: Cluster,
    certexporter_deployment: List[pykube.Deployment],
    certexporter_daemonset: List[pykube.DaemonSet],
    cert_manager_app_cr: ConfiguredApp,
):
    # patch services to be type: NodePort
    prepare_services(kube_cluster)

    # apply certificate
    kube_cluster.kubectl(
        "apply",
        filename=Path(request.fspath.dirname) / "cert-manager-certificate.yaml",
        output_format="",
    )

    # let the dust settle
    time.sleep(10)

    # Request metrics from the exporters and check if they match expected values
    metric_name = "cert_exporter_certificate_cr_not_after"

    def validate_cert_metric(m):
        return (
            ('issuer_ref="selfsigned-giantswarm"' in m)
            and ('managed_issuer="true",name="test-cert",namespace="default"' in m)
            and ('name="test-cert",namespace="default"' in m)
            and ('namespace="default"' in m)
        )

    # request from deployment port
    deploy_metrics = retrieve_metrics(deployment_port)
    assert_metric(deploy_metrics, metric_name, validate_cert_metric)

    # request from daemonset port
    ds_metrics = retrieve_metrics(daemonset_port)
    assert len([m for m in ds_metrics if m.startswith(metric_name)]) == 0
