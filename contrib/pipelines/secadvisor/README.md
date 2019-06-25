# Security Advisor Pipeline

Implements the IBM Security Advisor pipeline


## Setup Skydive

First build and run skydive:

```
make
cp etc/skydive.yml.default skydive.yml
sudo $(which skydive)) allineone -c skydive.yml
```

## Setup Minio

Install and run minio (an AWS S3-like object store).

## Setup Pipeline

Build and run the pipeline:

```
make all
make install
make run
```

## Generate Flows

Via the Skydive WebUI setup captures and generate traffic which should
result in the secadvisor pipeline sending flows to the ObjectStore.

