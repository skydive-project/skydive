# Using skydive to roll-out configuration changes

The following script demonstrates k8s automation with Skydive by using the
skydive client (which in turn makes use of skydive REST API).

Let assume the following graph nodes:

```
- secret:mysql-pass
- pod:wordpress-<hash>
- pod:wordpress-mysql-<hash>
```

And the following graph edges:

```
- pod:wordpress-<hash> --> secret:mysql-pass
- pod:wordpress-mysql-<hash> --> secret:mysql-pass
```

Assume that initially:

```
- secret:mysql-pass password: abc123
- pod:wordpress-<hash> password: abc123
- pod:wordpress-mysql-<hash> password: abc123
```

Now if we re-create secret with a new password: `abc456` we get:

```
- secret:mysql-pass password: abc456
- pod:wordpress-<hash> password: abc123
- pod:wordpress-mysql-<hash> password: abc123
```

Which is in-consistent, thus requiring that either we re-create pods manually
or use the script provided here to re-create pods:

```
$ ./skydive.sh -c <skydive-config-yaml> secret default/mysql-pass
```

Which will trigger automatically:

```
kubectl delete pod wordpress-<hash>
kubectl delete pod wordpress-mysql-<hash>
```

Triggering the deployment to re-create the pods, and pods will use the updated
secret object so as a result:

```
- secret:mysql-pass password: abc456
- pod:wordpress-<hash> password: abc456
- pod:wordpress-mysql-<hash> password: abc456
```

Which is as expected!
