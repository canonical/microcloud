# End to end testing of MicroCloud

Once MicroCloud is deployed, end to end testing can verify that everything works as it should.

```{caution}
By running end to end tests against a MicroCloud, you acknowledge that:

1. All instances will be impacted as the cluster member evacuations require them to be migrated (and restarted if not live-migrated).
1. Resources will be consumed (instances, memory, CPU, disk space, IP addresses, etc).

As such, those end to end tests are best used to ascertain the working condition of a MicroCloud deployment before  deploying production workload to it.
```

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

Upon successful completion, a confirmation prompt will be displayed before destroying the testing environment. This behavior can be changed by setting the `DESTROY` variable accordingly:

```sh
# Leave the environment in place
DESTROY=no ./run mc

# or automatically destroy it without prompting
DESTROY=yes ./run mc
```

```{important}
The client credentials used during tests are not removed from the tested MicroCloud environment at the end. This part of the cleanup needs to be done manually.
```

### Rolling reboots

During the test execution, each cluster member will be evacuated, rebooted and restored into the cluster. The evacuation and restoration is done by the test script itself but the machine reboot needs to be done externally.

The script will pause and ask the operator to proceed with rebooting the evacuated cluster member and wait for it to come back online before resuming the script execution.

````{note}
If desired and possible, the evacuated cluster member reboot can be automated by creating an executable file in the same directory as the `run` script. That executable needs to be named `reboot` and will be invoked:

```sh
# REMOTE: the name of the LXD remote used (i.e: the MicroCloud being tested)
# member: the name of the evacuated cluster member that needs to be rebooted
./reboot "${REMOTE}" "${member}"
```
````

It is possible to influence how many rolling reboots are performed:

```sh
# don't do any rolling reboot/evacuation test
EVACUATION_COUNTS=0 ./run mc

# do multuple rolling reboots/evacuation tests:
EVACUATION_COUNTS=5 ./run mc
```
