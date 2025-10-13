package main

import (
	"testing"

	cephTypes "github.com/canonical/microceph/microceph/api/types"
	microTypes "github.com/canonical/microcluster/v3/rest/types"
	"github.com/stretchr/testify/suite"

	"github.com/canonical/microcloud/microcloud/api/types"
)

type statusSuite struct {
	suite.Suite
}

func TestStatusSuite(t *testing.T) {
	suite.Run(t, new(statusSuite))
}

func (s *statusSuite) Test_statusWarnings() {
	genMember := func(name string, status microTypes.MemberStatus) microTypes.ClusterMember {
		return microTypes.ClusterMember{
			ClusterMemberLocal: microTypes.ClusterMemberLocal{Name: name},
			Status:             status,
		}
	}

	cases := []struct {
		desc             string
		statuses         []types.Status
		expectedWarnings Warnings
		expectedStatus   map[string]microTypes.MemberStatus
	}{
		{
			desc: "Single node MicroCloud with LXD",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.101",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline)},
					},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "Reliability risk: 3 systems are required for effective fault tolerance"},
				{Level: Warn, Message: "MicroOVN is not found on micro01"},
				{Level: Warn, Message: "MicroCeph is not found on micro01"},
			},
			expectedStatus: map[string]microTypes.MemberStatus{"micro01": microTypes.MemberOnline, "micro02": microTypes.MemberOnline},
		},
		{
			desc: "2 node MicroCloud with LXD",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.101",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "Reliability risk: 3 systems are required for effective fault tolerance"},
				{Level: Warn, Message: "MicroOVN is not found on micro01, micro02"},
				{Level: Warn, Message: "MicroCeph is not found on micro01, micro02"},
			},
			expectedStatus: map[string]microTypes.MemberStatus{"micro01": microTypes.MemberOnline, "micro02": microTypes.MemberOnline},
		},
		{
			desc: "2 node MicroCloud with LXD, member is not online",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.101",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", "some unknown status")},
					},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "Reliability risk: 3 systems are required for effective fault tolerance"},
				{Level: Warn, Message: "MicroOVN is not found on micro01, micro02"},
				{Level: Warn, Message: "MicroCeph is not found on micro01, micro02"},
				{Level: Error, Message: "LXD is not available on micro02"},
			},
			expectedStatus: map[string]microTypes.MemberStatus{"micro01": microTypes.MemberOnline, "micro02": "some unknown status"},
		},
		{
			desc: "2 node MicroCloud with LXD, member is upgrading",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.101",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberNeedsUpgrade)},
					},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "Reliability risk: 3 systems are required for effective fault tolerance"},
				{Level: Warn, Message: "MicroOVN is not found on micro01, micro02"},
				{Level: Warn, Message: "MicroCeph is not found on micro01, micro02"},
				{Level: Warn, Message: "LXD upgrade in progress"},
			},
			expectedStatus: map[string]microTypes.MemberStatus{"micro01": microTypes.MemberOnline, "micro02": microTypes.MemberNeedsUpgrade},
		},
		{
			desc: "2 node MicroCloud with LXD, offline node and upgrading node in the same cluster",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.101",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", "some unknown status"), genMember("micro02", microTypes.MemberNeedsUpgrade)},
					},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "Reliability risk: 3 systems are required for effective fault tolerance"},
				{Level: Warn, Message: "MicroOVN is not found on micro01, micro02"},
				{Level: Warn, Message: "MicroCeph is not found on micro01, micro02"},
				{Level: Error, Message: "LXD is not available on micro01"},
				{Level: Warn, Message: "LXD upgrade in progress"},
			},
			expectedStatus: map[string]microTypes.MemberStatus{"micro01": "some unknown status", "micro02": microTypes.MemberNeedsUpgrade},
		},
		{
			desc: "2 node MicroCloud with LXD, offline node and upgrading node across services",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.101",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberNeedsUpgrade), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", "some unknown status"), genMember("micro02", microTypes.MemberOnline)},
					},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "Reliability risk: 3 systems are required for effective fault tolerance"},
				{Level: Warn, Message: "MicroOVN is not found on micro01, micro02"},
				{Level: Warn, Message: "MicroCeph is not found on micro01, micro02"},
				{Level: Error, Message: "LXD is not available on micro01"},
				{Level: Warn, Message: "MicroCloud upgrade in progress"},
			},
			expectedStatus: map[string]microTypes.MemberStatus{"micro01": "some unknown status", "micro02": microTypes.MemberOnline},
		},
		{
			desc: "2 node MicroCloud with LXD, one node removed from LXD",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.101",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline)},
					},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "Reliability risk: 3 systems are required for effective fault tolerance"},
				{Level: Error, Message: "LXD is not found on micro02"},
				{Level: Error, Message: "MicroCloud members not found in LXD: micro02"},
				{Level: Warn, Message: "MicroOVN is not found on micro01, micro02"},
				{Level: Warn, Message: "MicroCeph is not found on micro01, micro02"},
			},
			expectedStatus: map[string]microTypes.MemberStatus{"micro01": microTypes.MemberOnline, "micro02": microTypes.MemberOnline},
		},
		{
			desc: "2 node MicroCloud with LXD and MicroCeph, one node removed from MicroCeph",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.101",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "Reliability risk: 3 systems are required for effective fault tolerance"},
				{Level: Error, Message: "MicroCloud members not found in MicroCeph: micro02"},
				{Level: Warn, Message: "MicroOVN is not found on micro01, micro02"},
				{Level: Warn, Message: "No MicroCeph OSDs configured"},
				{Level: Warn, Message: "MicroCeph is not found on micro02"},
			},
			expectedStatus: map[string]microTypes.MemberStatus{"micro01": microTypes.MemberOnline, "micro02": microTypes.MemberOnline},
		},
		{
			desc: "2 node MicroCloud with LXD and MicroCeph, no OSDs",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.101",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "Reliability risk: 3 systems are required for effective fault tolerance"},
				{Level: Warn, Message: "MicroOVN is not found on micro01, micro02"},
				{Level: Warn, Message: "No MicroCeph OSDs configured"},
			},
		},
		{
			desc: "2 node MicroCloud with LXD and MicroCeph, too few OSDs",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.101",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
					OSDs: cephTypes.Disks{{OSD: 0}},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "Reliability risk: 3 systems are required for effective fault tolerance"},
				{Level: Warn, Message: "Data loss risk: MicroCeph OSD replication recommends at least 3 disks across 3 systems"},
				{Level: Warn, Message: "MicroOVN is not found on micro01, micro02"},
			},
		},
		{
			desc: "2 node MicroCloud with LXD and MicroCeph, sufficient OSDs",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.101",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
					OSDs: cephTypes.Disks{{OSD: 2}},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
					OSDs: cephTypes.Disks{{OSD: 0}, {OSD: 1}},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "Reliability risk: 3 systems are required for effective fault tolerance"},
				{Level: Warn, Message: "MicroOVN is not found on micro01, micro02"},
			},
		},
		{
			desc: "2 node MicroCloud with LXD and MicroCeph. Overloaded MicroCeph",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.101",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
					OSDs: cephTypes.Disks{{OSD: 2}},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
					OSDs: cephTypes.Disks{{OSD: 0}, {OSD: 1}},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "Reliability risk: 3 systems are required for effective fault tolerance"},
				{Level: Warn, Message: "MicroOVN is not found on micro01, micro02"},
				{Level: Warn, Message: "Found MicroCeph systems not managed by MicroCloud: micro03"},
			},
		},
		{
			desc: "2 node MicroCloud with LXD, MicroCeph, and MicroOVN",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.101",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.MicroOVN:   {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.MicroOVN:   {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline)},
					},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "Reliability risk: 3 systems are required for effective fault tolerance"},
				{Level: Warn, Message: "No MicroCeph OSDs configured"},
			},
		},
		{
			desc: "3 node MicroCloud with LXD",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.100",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
					},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
					},
				},
				{
					Name:    "micro03",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
					},
				},
			},
			expectedWarnings: []Warning{
				{Level: Warn, Message: "MicroOVN is not found on micro01, micro02, micro03"},
				{Level: Warn, Message: "MicroCeph is not found on micro01, micro02, micro03"},
			},
		},
		{
			desc: "3 node MicroCloud with LXD, MicroCeph, MicroOVN, and sufficient OSDs (no warnings)",
			statuses: []types.Status{
				{
					Name:    "micro01",
					Address: "10.0.0.100",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.MicroOVN:   {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
					},
					OSDs: cephTypes.Disks{{OSD: 0}},
				},
				{
					Name:    "micro02",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.MicroOVN:   {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
					},
					OSDs: cephTypes.Disks{{OSD: 1}},
				},
				{
					Name:    "micro03",
					Address: "10.0.0.102",
					Clusters: map[types.ServiceType][]microTypes.ClusterMember{
						types.MicroCloud: {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.MicroOVN:   {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.MicroCeph:  {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
						types.LXD:        {genMember("micro01", microTypes.MemberOnline), genMember("micro02", microTypes.MemberOnline), genMember("micro03", microTypes.MemberOnline)},
					},
					OSDs: cephTypes.Disks{{OSD: 2}},
				},
			},
			expectedWarnings: []Warning{},
		},
	}

	for i, c := range cases {
		s.T().Log(i, c.desc)
		warnings := compileWarnings("micro01", c.statuses)

		s.Len(warnings, len(c.expectedWarnings))
		for _, w := range warnings {
			s.Contains(c.expectedWarnings, w)
		}

		for _, row := range c.statuses {
			rows := formatStatusRow(c.statuses[0], row)
			status := rows[len(rows)-1]

			if c.expectedStatus != nil {
				expectedStatus, ok := c.expectedStatus[row.Name]
				s.True(ok)
				s.Equal(string(expectedStatus), status)
			}
		}
	}
}
