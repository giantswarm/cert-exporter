import logging
import requests
import time
from functools import partial
from json import dumps
from typing import Dict, List

import pykube
import pytest
from pytest_helm_charts.fixtures import Cluster
from pytest_helm_charts.utils import (
    wait_for_deployments_to_run,
    wait_for_daemon_sets_to_run,
)

logger = logging.getLogger(__name__)

app_name = "cert-exporter"
namespace_name = "kube-system"
catalog_name = "chartmuseum"

timeout: int = 360


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
    return wait_for_deployment(kube_cluster)


def wait_for_deployment(kube_cluster: Cluster) -> List[pykube.Deployment]:
    deployments = wait_for_deployments_to_run(
        kube_cluster.kube_client,
        ["secret-cert-exporter"],
        namespace_name,
        timeout,
    )
    return deployments


@pytest.fixture(scope="module")
def certexporter_daemonset(kube_cluster: Cluster) -> List[pykube.DaemonSet]:
    daemonsets = wait_for_daemon_sets_to_run(
        kube_cluster.kube_client,
        ["cert-exporter"],
        namespace_name,
        timeout,
    )
    return daemonsets

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

    # let the dust settle
    time.sleep(10)

    ports = [
        30017,
        30018,
        30027,
        30028,
    ]  # change this if your want to change the kind config

    for port in ports:
        r = requests.get(f"http://127.0.0.1:{port}/metrics")

        assert r.status_code == 200
