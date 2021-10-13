# import sys
# sys.path.append(os.basename(sys.argv[0]))

import logging
import requests
import time
from functools import partial
from json import dumps
from pathlib import Path
from typing import Dict, List

import pykube
import pytest
from pytest_helm_charts.fixtures import Cluster
from pytest_helm_charts.utils import (
    wait_for_deployments_to_run,
    wait_for_daemon_sets_to_run,
)
from pytest_helm_charts.giantswarm_app_platform.app import (
    AppFactoryFunc,
    ConfiguredApp,
)

# from pytest_helm_charts.giantswarm_app_platform.custom_resources import AppCR

from cert_util import cert_gen


logger = logging.getLogger(__name__)

app_name = "cert-exporter"
namespace_name = "kube-system"
cert_manager_app_chart_version = "2.11.0"
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
    kubectl("patch service cert-exporter", patch=ce_patch)
    logger.info("Patched cert-exporter service")

    sce_patch = dumps(
        {
            "spec": {
                "type": "NodePort",
                "ports": [
                    {"name": "secret-cert-exporter", "port": 9005, "nodePort": 30008}
                ],
            }
        }
    )
    kubectl("patch service secret-cert-exporter", patch=sce_patch)
    logger.info("Patched secret-cert-exporter service")


@pytest.mark.smoke
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
def test_cluster_info(
    kube_cluster: Cluster, cluster_type: str, chart_extra_info: Dict[str, str]
) -> None:
    """Example shows how you can access additional information about the cluster the tests are running on"""
    logger.info(f"Running on cluster type {cluster_type}")
    key = "external_cluster_type"
    if key in chart_extra_info:
        logger.info(f"{key} is {chart_extra_info[key]}")
    assert kube_cluster.kube_client is not None
    assert cluster_type != ""


@pytest.fixture(scope="module")
def certexporter_deployment(kube_cluster: Cluster) -> List[pykube.Deployment]:
    deployments = wait_for_deployments_to_run(
        kube_cluster.kube_client,
        ["secret-cert-exporter"],
        namespace_name,
        timeout,
    )
    return deployments


@pytest.fixture(scope="module")
def cert_manager_app_cr(app_factory: AppFactoryFunc) -> ConfiguredApp:
    res = app_factory(
        "cert-manager-app",
        cert_manager_app_chart_version,
        "giantswarm-stable",
        "https://giantswarm.github.io/giantswarm-catalog/",
        namespace="cert-manager-app",
        timeout_sec=timeout,
        deployment_namespace="cert-manager-app",
    )
    return res


@pytest.fixture(scope="module")
def certexporter_daemonset(kube_cluster: Cluster) -> List[pykube.DaemonSet]:
    daemonsets = wait_for_daemon_sets_to_run(
        kube_cluster.kube_client,
        ["cert-exporter"],
        namespace_name,
        timeout,
    )
    return daemonsets


def retrieve_metrics(port: int) -> List[str]:
    r = requests.get(f"http://127.0.0.1:{port}/metrics")

    assert r.status_code == 200

    raw_metrics = r.text

    metrics_lines = [metric for metric in raw_metrics.splitlines() if not metric.startswith("#")]

    assert len(metrics_lines) != 0

    return metrics_lines


def assert_metric(metrics: List[str], metric: str) -> None:
    # found = [m for m in cp_ds_metrics if m.startswith()]
    found = [m for m in metrics if m.startswith(metric)]
    assert len(found) == 1


@pytest.mark.smoke
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
def test_file_certificate_metrics(
    request,
    kube_cluster: Cluster,
    certexporter_deployment: List[pykube.Deployment],
    certexporter_daemonset: List[pykube.DaemonSet],
):
    # patch services to be type: NodePort
    prepare_services(kube_cluster)

    metric_name = "cert_exporter_not_after"

    mount_dir = Path(f"{request.fspath.dirname}/kind-mounts/control-plane")

    cert_name = "control-plane-cert"

    # Create certificate file on node
    logger.info(f"Writing certificate file to {mount_dir}")
    cert_gen(name=cert_name, target_dir=mount_dir)

    # let the dust settle
    time.sleep(5)

    # request from daemonset port
    ds_metrics = retrieve_metrics(daemonset_port)
    logger.info(ds_metrics)
    assert_metric(ds_metrics, f"{metric_name}{{path=\"/certs/{cert_name}.crt\"}}")

    # request from deployment port
    deploy_metrics = retrieve_metrics(deployment_port)
    logger.info(deploy_metrics)
    assert len([m for m in deploy_metrics if m.startswith(metric_name)]) == 0




# @pytest.mark.functional
# def test_certificate_cr_metrics(
#     request,
#     kube_cluster: Cluster,
#     certexporter_deployment: List[pykube.Deployment],
#     certexporter_daemonset: List[pykube.DaemonSet],
#     cert_manager_app_cr: ConfiguredApp,
# ):
#     # patch services to be type: NodePort
#     prepare_services(kube_cluster)
#
#
#     # Create cert-manager certificate and
#
#     # Create a kubernetes.io/tls secret
#
#     # Request metrics from the exporters and check if they match expected values
#
