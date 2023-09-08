# Setup : How to setup a test MAAS cluster with Juju and a custom image server

## (Optional) Local image server

If you have a flaky internet connection, you can setup a local image server to locally serve the images to MAAS.

```bash
cd local-maas-image-server && ./run.sh
```

You should now have a local image server (only serving Jammy/amd64 images for the sake of this tutorial) running at `0.0.0.0:5000`. When MAAS is setup, you can go to the MAAS UI, go to `Images` and add your new image server with the following URL: `http://0.0.0.0:5000/`. No need to wait for `images.maas.io` which I found quite slow..

## MAAS setup with Juju and charm deployment

You can follow the instructions in `maas-setup.sh`
