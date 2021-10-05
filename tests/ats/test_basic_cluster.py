import logging
import requests
import time
from contextlib import contextmanager
from functools import partial
from json import dumps
from pathlib import Path
from typing import Dict, List, Optional

import pykube
import pytest
from pytest_helm_charts.fixtures import Cluster
from pytest_helm_charts.utils import wait_for_deployments_to_run

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


# scope "module" means this is run only once, for the first test case requesting! It might be tricky
# if you want to assert this multiple times
@pytest.fixture(scope="module")
def certexporter_deployment(kube_cluster: Cluster) -> List[pykube.Deployment]:
    return wait_for_deployment(kube_cluster)


def wait_for_deployment(kube_cluster: Cluster) -> List[pykube.Deployment]:
    deployments = wait_for_deployments_to_run(
        kube_cluster.kube_client,
        [app_name],
        namespace_name,
        timeout,
    )
    return deployments


def try_ingress(port, host, expected_status):
    retries = 5
    last_status = 0
    while retries != 0:
        logger.info(f"trying GET http://127.0.0.1:{port}/ with Host: {host}")
        try:
            r = requests.get(f"http://127.0.0.1:{port}/", headers={"Host": host})
            last_status = r.status_code
        except Exception as e:
            logger.info(f"Request failed: {e}")
        else:
            logger.info(f"Result: {last_status}. Expected {expected_status}")

        retries = retries - 1
        if retries == 0:
            break

        time.sleep(1)

    return last_status == expected_status


# when we start the tests on circleci, we have to wait for pods to be available, hence
# this additional delay and retries
@pytest.mark.smoke
@pytest.mark.flaky(reruns=5, reruns_delay=10)
def test_pods_available(kube_cluster: Cluster, ic_deployment: List[pykube.Deployment]):
    for s in ic_deployment:
        assert int(s.obj["status"]["readyReplicas"]) > 0


# @pytest.mark.functional
# def test_ingress_creation(
#     request, kube_cluster: Cluster, ic_deployment: List[pykube.Deployment]
# ):
#     kube_cluster.kubectl("apply", filename=Path(request.fspath.dirname) / "test-ingress.yaml", output_format="")

#     kube_cluster.kubectl(
#         "wait deployment helloworld --for=condition=Available",
#         timeout="60s",
#         output_format="",
#         namespace="helloworld",
#     )

#     # try the ingress
#     retries = 10
#     last_status = 0
#     while last_status != 200:
#         r = requests.get("http://127.0.0.1:8080/", headers={"Host": "helloworld"})
#         last_status = r.status_code

#         if last_status == 200 or retries == 0:
#             break

#         retries = retries - 1
#         time.sleep(5)

#     assert last_status == 200


# @pytest.mark.functional
# @pytest.mark.flaky(reruns=5, reruns_delay=10)
# def test_multiple_ingress_controllers(
#     request, kube_cluster: Cluster, ic_deployment: List[pykube.Deployment], chart_version
# ):
#     logger.info("applying manifests")
#     # apply test manifests
#     kube_cluster.kubectl(
#         "apply", filename=Path(request.fspath.dirname) / "multi-controller-manifests.yaml", output_format=""
#     )

#     # shortcut function with fixed namespace
#     # (defined in multi-controller-manifests.yaml)
#     kubectl = partial(
#         kube_cluster.kubectl, namespace="second-ingress-controller", output_format=""
#     )
#     logger.info("patching app with current chart version")
#     # patch the app cr with the right version
#     patch = dumps([{"op": "replace", "path": "/spec/version", "value": chart_version}])
#     kubectl(f"patch app second-ingress-controller --type=json", patch=patch)

#     logger.info("waiting until second controller deployment is ready")
#     wait_for_deployments_to_run(
#         kube_cluster.kube_client,
#         ["second-ingress-controller"],
#         "second-ingress-controller",
#         timeout,
#     )

#     logger.info("waiting until second helloworld deployment is ready")
#     wait_for_deployments_to_run(
#         kube_cluster.kube_client,
#         ["helloworld-2"],
#         "helloworld-2",
#         timeout,
#     )

#     logger.info("Checking if controller handle their respective Ingresses")
#     # try the ingresses and expect 404 or 200 on port 8080 and 8081
#     assert try_ingress(8081, "helloworld-2", 200)
#     assert try_ingress(8081, "helloworld", 404)

#     # try the ingress on port 8080 and expect 404
#     assert try_ingress(8080, "helloworld-2", 404)
#     assert try_ingress(8080, "helloworld", 200)
