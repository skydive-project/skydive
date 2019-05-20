# Dummy Pipeline

Is a simple pipeline which logs all captures flows

To use it build and run skydive:

```
make
cp etc/skydive.yml.default skydive.yml
sudo $(which skydive)) allineone -c skydive.yml
```

Then build and run the pipeline:

```
make all
make install
make run
```

Finally via the Skydive WebUI setup captures and generate traffic which should
result in the dummy pipeline outputting flows onto the console.
