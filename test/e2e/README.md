# End to end testing of MicroCloud

Once MicroCloud is deployed, end to end testing can verify that everything works as it should.

## Testing

The [`run`](run) script will setup a dedicated project using the [Terraform provider for LXD](https://github.com/terraform-lxd/terraform-provider-lxd) and deploy a number of instances on the MicroCloud. Those instances will be a mix of containers and virtual machines.

It is possible to override the variables defined in [`variables.tf`](variables.tf) by creating a file named `terraform.tfvars`:

```sh
# terraform.tfvars
containers_per_host = 25
vms_per_host = 5
```

Then execute the script and provide it the name of the LXD remote corresponding to the MicroCloud deployment to test (`mc` here):

```sh
./run mc
```

If the LXD remote to test is not configured yet, it's possible to add it with an identity token issued from a MicroCloud cluster member (`micro1` here):

```sh
# Create the needed group
root@micro1:~# lxc auth group create e2e-testing
Group e2e-testing created

# Grant admin permission on server
root@micro1:~# lxc auth group permission add e2e-testing server admin

# Issue the identity token
root@micro1:~# lxc auth identity create tls/e2e-testing
TLS identity "tls/e2e-testing" (d16614b8-f39e-4ef5-8e9b-90764b61b660) pending identity token:
eyJjbGllbnRfbmFtZSI6ImUyZS10ZXN0aW5nIiwiZmluZ2VycHJpbnQiOiIyNzY1OGNjYmRlNmRkZDcwNDliMzliNmY5NzA1MWIzMWVkMWQ0NDM2OTFjZGFjMWIxNmI1ODBjNmI4NzFiNmM2IiwiYWRkcmVzc2VzIjpbIjE3Mi4yNC4yNi4xODo4NDQzIiwiWzIwMDE6NDcwOmIxYzM6Nzk0Njo4NWYzOmQzYTg6ZDcxZTo3YjA1XTo4NDQzIl0sInNlY3JldCI6ImJjNjlmOTM4YzM5NjRhNTI1ZDI3OTlmMTEyM2Q2NWE5ZGExZDE1ZGFmNjVjZjlmNTViYjQ1ZDdmZjBiMjg1YmYiLCJleHBpcmVzX2F0IjoiMDAwMS0wMS0wMVQwMDowMDowMFoiLCJ0eXBlIjoiQ2xpZW50IGNlcnRpZmljYXRlIn0=

# Add the freshly created identity to the freshly created group
root@micro1:~# lxc auth identity group add tls/e2e-testing e2e-testing
```

This token can be used on the machine where the `run` script will be used:

```sh
lxc remote add mc https://microcloud.lxd:8443 --token eyJjbGllbnRfbmFtZSI6ImUyZS10ZXN0aW5nIiwiZmluZ2VycHJpbnQiOiIyNzY1OGNjYmRlNmRkZDcwNDliMzliNmY5NzA1MWIzMWVkMWQ0NDM2OTFjZGFjMWIxNmI1ODBjNmI4NzFiNmM2IiwiYWRkcmVzc2VzIjpbIjE3Mi4yNC4yNi4xODo4NDQzIiwiWzIwMDE6NDcwOmIxYzM6Nzk0Njo4NWYzOmQzYTg6ZDcxZTo3YjA1XTo4NDQzIl0sInNlY3JldCI6ImJjNjlmOTM4YzM5NjRhNTI1ZDI3OTlmMTEyM2Q2NWE5ZGExZDE1ZGFmNjVjZjlmNTViYjQ1ZDdmZjBiMjg1YmYiLCJleHBpcmVzX2F0IjoiMDAwMS0wMS0wMVQwMDowMDowMFoiLCJ0eXBlIjoiQ2xpZW50IGNlcnRpZmljYXRlIn0=


./run mc
```
