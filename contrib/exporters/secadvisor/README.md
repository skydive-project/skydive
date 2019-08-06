# Deploy Security Advisor Pipeline

The IBM Security Advisor pipeline extracts flow data from skydive, filters out
external traffic, transforms flow records by adding status information, and
flow classification (egress, ingress, internal), batches flows records together
into streams (by time or count), then encodes these blocks in JSON form, and
compresses using GZIP and finally stores as objects into any S3 compatible
target.

## Deploy Skydive

Follow instructions on http://skydive.network/documentation/build to install prerequisites and prepare your machine to build the skydive code.
Build and run skydive:

```
make static
sudo $(which skydive) allineone -c etc/skydive.yml.default
```

## Setup Minio

For running tests on a local machine, setup a local Minio object store.

First run the minio server:

```
wget https://dl.min.io/server/minio/release/linux-amd64/minio
chmod +x minio
mkdir /tmp/data
export MINIO_VOLUMES="/var/lib/minio"
export MINIO_ACCESS_KEY=user
export MINIO_SECRET_KEY=password
./minio server /tmp/data
```

Next create the bucket (as destination location for flows):

```
wget https://dl.minio.io/client/mc/release/linux-amd64/mc
chmod +x mc
./mc config host add local http://localhost:9000 user password /var/lib/minio --api S3v4
./mc rm --recursive --force local/bucket
./mc rb --force local/bucket
./mc mb --ignore-existing local/bucket
```

## Deploy the Pipeline

Build and run the pipeline in the secadvisor directory:

```
cd contrib/pipelines/secadvisor
make static
./secadvisor secadvisor.yml.default
```

## Create Capture

Setup capture on every interface:

```
skydive client capture create \
	--gremlin "G.V().Has('Type', 'device', 'Name', 'eth0')" \
	--type pcap \
	-c etc/skydive.yml.default
```

## Inject Traffic

```
skydive client inject-packet create --count 1000 --type tcp4 \
	--src "G.V().Has('Type', 'device', 'Name', 'eth0')" \
	--dst "G.V().Has('Type', 'device', 'Name', 'eth0')" \
	-c etc/skydive.yml.default
```

## Verify Operation

Wait for 60 seconds and then check that we have object accumulate in the bucket
as a result of the operation.

```
./mc ls --recursive local/bucket
```
