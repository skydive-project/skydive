# Security Advisor Pipeline

Implements the IBM Security Advisor pipeline

The security advisor filters the flow data obtained from skydive, performs a data transformation, and saves the information to an object store.
This data may then be used to perform various kinds of analysis for security, accounting, or other purposes.

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


## Setup Object Store

The security advisor saves data to an S3-type object store.
The parameters to access the object store must be provided in the secadvisor.yml configuration file.
In case the user does not already have an object store, we show below how to create an object store for testing purposes.
We show how to create either a minio object store running locally or an IBM Cloud Object Store.

### Setup Minio

Follow the instructions on https://docs.min.io/docs/minio-quickstart-guide.html to install and run a minio object store (an AWS S3-like object store).

```
wget https://dl.min.io/server/minio/release/linux-amd64/minio
chmod +x minio
./minio server /data
```

This will run a basic minio object store server on your machine, placing the objects in "/data".
Note the printed output in the logs from the ./minio command and take down the values of "Endpoint", "AccessKey", and "SecretKey". Place these items in the secadvisor.yml in the appropriate locations (see below). 
Create a bucket in the object store to hold the security-advisor data.

### Setup IBM COS

Other s3-compatible object stores may be used, such as IBM COS.

In IBM Cloud, Create an Object Store resource. The Lite plan gives limited resources for free.
In the Object Store, create a bucket to hold the Skydive flow information. For simple testing, you can choose Single Site resiliency.
Look under Bucket Configuration to see the Public endpoint (url) where the bucket is accessed. This endpoint information needs to go into the secadvisor.yml "endpoint" field.
Go to the "Service Credentials" panel and create a new credential. Click on "View Credential" to get the details of the credential. The apikey needs to go into the secadvisor.yml "api_key" field.
In the secadvisor.yml file, uncomment the iam_endpoint field and set it to https://iam.cloud.ibm.com/identity/token.

### Ceph, AWS

TBD

## Prepare secadvisor.yml

```
cp secadvisor.yml.default secadvisor.yml
```

Set the fields "endpoint", "access_key", "secret_key", "bucket", "cluster_net_masks", etc.
If using IBM COS, then you may provide <"api_key", "iam_endpoint"> instead of <"access_key", "secret_key">.
The subnets specified in cluster_net_masks are used to determine whether a flow is internal, ingress, or egress. Enter in the list all of the subnets that are recognized as being inside your cluster.
The types of "excluded_tags" that are recognized are: internal, ingress, egress, other.

Be sure to set the parameters max_flow_array_size, etc, to reasonable values.

The max_flow_array_size specifies the maximum number of flows that will be stored in each iteration.
If max_flow_array_size is set to 0 (or not set at all), then the number of flows that can be stored is 0, and hence no useful information will be saved in the object store.
This will result in an error.

The max_flows_per_object parameter specifies the number of flows that may stored in a single object. If there are more flows found in a single iteration, then several objects will be created, each with up to max_flows_per_object flows in them. In order to have all the flows stored in a single object, be sure that this parameter is set sufficiently large.
If max_flows_per_object is set to 0 (or not set at all), then each object will be able to hold 0 flows; i.e. no flows will be able to be stored in any object.
This will result in an error.

The flow information is collected in groups of objects (called streams) determined by the max_seconds_per_stream parameter. All the objects generated within the number of seconds specified in the max_seconds_per_stream parameter are collected under the same heading (stream)  in the object url path.
If max_seconds_per_stream is set to a very large number, then all of the flows captured in the current run of the security advisor will be included in the same collection (stream)

## Setup Pipeline

Build and run the pipeline in the secadvisor directory:

```
cd $SKYDIVE_BASE/contrib/pipelines/secadvisor
make all
make install
make run
```

or

```
make
secadvisor secadvisor.yml
```

The "make all" compiles the source code and creates the secadvisor binary file in the same directory.
The "make run" runs the secadvisor binary using the secadvisor.yml.default that is provided with the source code.
This will run the secadvisor pipeline, which will ultimately fail to write to the object store,
unless the user first defines valid object store parameters in the secadvisor.yml file.

## Generate Flows

Via the Skydive WebUI setup captures and generate traffic which should
result in the secadvisor pipeline sending flows to the ObjectStore.

For example, run some iperf traffic between entities and capture the flow via the Skydive GUI.
Objects are created in the Object Store only if there is flow information to be saved.
Check your bucket in Object Store and see a new object about once per minute.
Stop the secadvisor or stop captures in the GUI to stop the creation of objects in the object store.



## Multiple pipelines

It is possible to run multiple pipelines simultaneously. Prepare a separate <n>.yml for each set of subnets or whatever configuration you want. Then run a separate instance of the security advisor (or other pipeline) using each of the <n>.yml configuration files. In this way it is possible to perform different filtering for different purposes or tenants.

## Trouble-shooting common problems

If the secadvisor log shows a "connection refused" error, verify that the proper address of the skydive analyzer is specified under "analyzers".

If the secadvisor log shows a credentials error, verify that the credential fields (<"access_key", "secret_key"> for minio, and <"api_key", "iam_endpoint"> for IBM COS) are properly set in the secadvisor.yml file.

If the secadvisor log shows an overflow and states that flows were discarded, check that max_flow_array_size is defined to some reasonable positive number.


## How to run tests

TBD



