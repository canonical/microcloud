#!/bin/bash

# Terraform demo configuration 
VMS="micro1 micro2 micro3 micro4"
NETWORK="microbr0"
VOLUMES="local1 local2 local3 local4 remote1 remote2 remote3"
STORAGE_POOL="default"
VM_COUNT=4
CEPH_EXCLUDED_VM="micro4"
OVN_INTERFACE="enp6s0"
OVN_IPV4_GATEWAY="192.0.2.1/24"
OVN_IPV4_RANGE="192.0.2.100-192.0.2.254"
LOCAL_DISKS="local1,local2,local3,local4"
CEPH_DISKS="remote1,remote2,remote3"

terraform_demo_cleanup() {
  echo "==> Terraform demo cleanup starting"
  
  lxc remote switch local || true
  
  # Try to destroy terraform resources if terraform directory exists
  if [ -d "/tmp/terraform_demo_test" ]; then
    echo "==> Attempting Terraform destroy"
    cd /tmp/terraform_demo_test || true
    terraform destroy -auto-approve || true
    cd - || true
    rm -rf /tmp/terraform_demo_test || true
  fi
  
  # Manual cleanup of any remaining demo-specific resources
  echo "==> Manual cleanup of terraform-created resources"
  
  # Remove VMs created by the terraform demo
  echo "Removing terraform VMs"
  for vm in ${VMS}; do
    if lxc info "${vm}" >/dev/null 2>&1; then
      echo "Removing terraform-created VM: ${vm}"
      lxc delete "${vm}" --force || true
    fi
  done
  
  # Remove networks created by the terraform demo
  echo "Removing terraform networks"
  if lxc network info "${NETWORK}" >/dev/null 2>&1; then
    echo "Removing terraform-created network: ${NETWORK}"
    lxc network delete "${NETWORK}" || true
  fi
  
  # Remove storage volumes created by the terraform demo
  echo "Removing terraform storage volumes"
  for vol in ${VOLUMES}; do
    if lxc storage volume info "${STORAGE_POOL}" "${vol}" >/dev/null 2>&1; then
      echo "Removing terraform-created storage volume: ${vol}"
      lxc storage volume delete "${STORAGE_POOL}" "${vol}" || true
    fi
  done
  
  echo "==> Terraform demo cleanup completed"
}

test_terraform_demo() {
  echo "Testing Terraform demo deployment workflow"
  
  # Set up cleanup trap to ensure cleanup happens even on failure
  trap terraform_demo_cleanup EXIT
  
  lxc remote switch local
  
  # Copy terraform demo to a test location to avoid modifying the original
  cp -r ../demos/terraform /tmp/terraform_demo_test
  cd /tmp/terraform_demo_test || exit 1
  
  # Use the exact same configuration as the example to test what users would actually use
  cp terraform.tfvars.example terraform.tfvars
  
  echo "==> Initializing Terraform"
  terraform init
  
  echo "==> Applying Terraform deployment"
  terraform apply -auto-approve
  
  # Wait for deployment to complete - terraform demo includes cloud-init setup
  echo "==> MicroCloud initialization completed"

  # Check that all VMs are running 
  echo "==> Verifying all VMs are running"
  running_vms=$(lxc list -f csv -c ns | grep -cxE 'micro[1-4],RUNNING')
  if [ "${running_vms}" -ne "${VM_COUNT}" ]; then
    echo "ERROR: Expected ${VM_COUNT} running VMs, found ${running_vms}"
    lxc list micro  # Show current state for debugging
    exit 1
  fi
  
  echo "==> Validating MicroCloud cluster health and configuration"
  if ! lxc exec micro1 -- microcloud status | grep -q "HEALTHY"; then
    echo "ERROR: MicroCloud cluster is not healthy"
    exit 1
  fi
  echo "MicroCloud cluster is healthy"
  for m in ${VMS}; do
    echo "  -> Validating LXD configuration for ${m}"
    # Validate LXD configuration using variables
    # Parameters: name, num_peers, local_disk, remote_disks, cephfs, ovn_interface, ipv4_gateway, ipv4_ranges, ipv6_gateway, dns_nameservers, profile_pool
    if ! validate_system_lxd "${m}" "${VM_COUNT}" "${LOCAL_DISKS}" 3 0 "${OVN_INTERFACE}" "${OVN_IPV4_GATEWAY}" "${OVN_IPV4_RANGE}" "" "" "${STORAGE_POOL}"; then
      echo "ERROR: LXD validation failed for ${m}"
      exit 1
    fi
    echo "  -> LXD validation passed for ${m}"
    
    # Validate MicroCeph configuration - only first 3 nodes have Ceph storage
    # Parameters: name, cephfs, encrypt, cluster_subnet, public_subnet, disks
    if [ "${m}" != "${CEPH_EXCLUDED_VM}" ]; then
      echo "  -> Validating MicroCeph configuration for ${m}"
      if ! validate_system_microceph "${m}" 0 0 "" "" "${CEPH_DISKS}"; then
        echo "ERROR: MicroCeph validation failed for ${m}"
        exit 1
      fi
      echo "  -> MicroCeph validation passed for ${m}"
    else
      echo "  -> Skipping MicroCeph validation for ${m} (excluded from Ceph storage)"
    fi
    
    # Validate MicroOVN configuration 
    echo "  -> Validating MicroOVN configuration for ${m}"
    if ! validate_system_microovn "${m}"; then
      echo "ERROR: MicroOVN validation failed for ${m}"
      exit 1
    fi
    echo "  -> MicroOVN validation passed for ${m}"
  done
  
  # Test basic cluster functionality by creating and managing a container
  echo "==> Testing basic cluster functionality"
  if ! lxc exec micro1 -- lxc launch ubuntu-minimal-daily:24.04 test-container; then
    echo "ERROR: Failed to launch test container"
    exit 1
  fi
  echo "  -> Test container launched successfully"

  # Wait for container to be ready and then delete it
  if ! lxc exec micro1 -- sh -c "$(declare -f waitInstanceReady); waitInstanceReady test-container"; then
    echo "ERROR: Test container failed to become ready"
    lxc exec micro1 -- lxc delete test-container --force || true
    exit 1
  fi
  echo "  -> Test container is ready"
  
  if ! lxc exec micro1 -- lxc delete test-container --force; then
    echo "ERROR: Failed to delete test container"
    exit 1
  fi
  echo "  -> Test container deleted successfully"
  
  echo "==> Terraform demo deployment test passed"
  
  # Test that terraform destroy works properly (this will be called again in cleanup trap)
  echo "==> Testing Terraform destroy functionality"
  terraform destroy -auto-approve
  
  # Verify cleanup worked
  echo "==> Verifying terraform destroy worked"
  for vm in ${VMS}; do
    if lxc info "${vm}" >/dev/null 2>&1; then
      echo "ERROR: VM ${vm} was not properly cleaned up by terraform destroy"
      exit 1
    fi
  done
  
  # Verify networks are cleaned up
  if lxc network info "${NETWORK}" >/dev/null 2>&1; then
    echo "ERROR: Network ${NETWORK} was not properly cleaned up by terraform destroy"
    exit 1
  fi
  
  echo "==> Terraform demo cleanup test passed"
  echo "==> Terraform demo test completed successfully"
  
  # Clear the trap since we completed successfully
  trap - EXIT
  terraform_demo_cleanup
}
