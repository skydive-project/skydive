# Security Advisor Pipeline

Implements the IBM Security Advisor pipeline


## Setup Skydive

Follow instructions on http://skydive.network/documentation/build to install prerequisites and prepare your machine to build the skydive code.
Build and run skydive:

```
make
cp etc/skydive.yml.default skydive.yml
// adjust settings in skydive.yml
sudo $(which skydive) allineone -c skydive.yml
```

Alternatively, run a skydive agent on each of your hosts and a skydive analyzer on one host.

On each machine:

```
sudo skydive agent -c skydive.yml
```

Be sure to set the field "analyzers" in the skydove.yml to point to the analyzer.

On one machine:

```
sudo skydive analyzer -c skydive.yml
```


## Setup Minio

Follow the instructions on https://docs.min.io/docs/minio-quickstart-guide.html to install and run a minio object store (an AWS S3-like object store).

```
wget https://dl.min.io/server/minio/release/linux-amd64/minio
chmod +x minio
./minio server /data
```

This will run a basic minio object store server on your machine, placing the objects in "/data".
Note the printed output in the logs from the ./minio command and take down the values of "Endpoint", "AccessKey", and "SecretKey". Place these items in the secadvisor.yml in the appropriate locations (see below). 
Create a bucket in the object store to hold the security-advisor data.

## Prepare secadvisor.yml

```
cp secadvisor.yml.default secadvisor.yml
```

Set the fields "endpoint", "access_key", "secret_key", "bucket", "cluster_net_masks", etc.
The subnets specified in cluster_net_masks are used to determine whether a flow is internal, ingress, or egress. Enter in the list all of the subnets that are recognized as being inside your cluster.
The types of "excluded_tags" that are recognized are: internal, ingress, egress, other.

## Setup Pipeline

Build and run the pipeline in the secadvisor directory:

```
make all
make install
make run
```

or

```
secadvisor secadvisor.yml
```

## Generate Flows

Via the Skydive WebUI setup captures and generate traffic which should
result in the secadvisor pipeline sending flows to the ObjectStore.

For example, run some iperf traffic between entities and capture the flow via the Skydive GUI.
Objects are created in the Object Store only if there is flow information to be saved.
Check your bucket in Object Store and see a new object about once per minute.
Stop the secadvisor or stop captures in the GUI to stop the creation of objects in the object store.


## Setup IBM COS

Other s3-compatible object stores may be used, such as IBM COS.

In IBM Cloud, Create an Object Store resource. The Lite plan gives limited resources for free.
In the Object Store, create a bucket to hold the Skydive flow information. For simple testing, you can choose Single Site resiliency.
Look under Bucket Configuration to see the Public endpoint (url) where the bucket is accessed. This endpoint information needs to go into the secadvisor.yml "endpoint" field.
Go to the "Service Credentials" panel and create a new credential. Click on "View Credential" to get the details of the credential. The apikey needs to go into the secadvisor.yml "api_key" field.
In the secadvisor.yml file, uncomment the iam_endpoint field and set it to https://iam.cloud.ibm.com/identity/token.


## Multiple pipelines

It is possible to run multiple security-advisor pipelines simultaneously. Prepare a separate secadvisor<n>.yml for each set of subnets or whatever configuration you want. Then run a separate instance of the security advisor using each of the secadvisor<n>.yml configuration files. In this way it is possible to perform different filtering for different purposes or tenants.

## Trouble-shooting common problems

If the secadvisor log shows a "connection refused" error, verify that the proper address of the skydive analyzer is specified under "analyzers".

If the secadvisor log shows a credentials error, verify that the credential fields (<"access_key", "secret_key"> for minio, and <"api_key", "iam_endpoint"> for IBM COS) are properly set in the secadvisor.yml file.



