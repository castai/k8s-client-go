#!/bin/bash

set -e

local_img=localhost:5000/conformance:$(date +%s)
img="${IMG:-$local_img}"

if [ "$IMG" == "" ]; then
    GOOS=linux go build -ldflags "-s -w" -o bin/conformance .
    docker build -t ${img} .
    docker push ${img}
fi

function log()
{
    echo "Test failed!"
    echo "Pods:"
    kubectl get pods -owide -n conformance
    echo "Logs:"
    kubectl logs -l app=conformance -n conformance
}
trap log ERR

kubectl delete ns conformance || true
kubectl create ns conformance
kubectl apply -f job.yaml --dry-run=client -oyaml | sed "s/replace-img/$(echo "$img" | sed 's/\//\\\//g')/" | kubectl apply -f - -n conformance
kubectl wait --for=condition=complete --timeout=10s job/conformance -n conformance
