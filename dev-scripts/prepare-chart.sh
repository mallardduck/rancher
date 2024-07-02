#!/bin/bash

test_image="rancher/rancher"
test_image_tag="v2.8.0"
sed -i -e "s/%VERSION%/${test_image_tag}/g" ./chart/Chart.yaml
sed -i -e "s/%APP_VERSION%/${test_image_tag}/g" ./chart/Chart.yaml
sed -i -e "s@%POST_DELETE_IMAGE_NAME%@${test_image}@g" ./chart/values.yaml
sed -i -e "s/%POST_DELETE_IMAGE_TAG%/${test_image_tag}/g" ./chart/values.yaml

echo "The chart can now be used with helm to install Rancher"