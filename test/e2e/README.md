# End to end testing of MicroCloud

Once MicroCloud is deployed, end to end testing can verify that everything works as it should.

```{caution}
By running end to end tests against a MicroCloud, you acknowledge that:

1. All instances will be impacted as the cluster member evacuations require them
   to be migrated (and restarted if not live-migrated).
1. Resources will be consumed (instances, memory, CPU, disk space, IP addresses,
   etc).

As such, these end to end tests are best used to ascertain the working condition
of a MicroCloud deployment before deploying production workloads to it.
```

## Overview

This suite is a [Ginkgo](https://onsi.github.io/ginkgo/)/[Gomega](https://onsi.github.io/gomega/)
Go test suite.
