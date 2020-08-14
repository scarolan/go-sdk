//
// Author:: Salim Afiune Maya (<afiune@lacework.net>)
// Copyright:: Copyright 2020, Lacework Inc.
// License:: Apache License, Version 2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package cmd

import (
	"sort"
	"strings"

	"github.com/lacework/go-sdk/api"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	vulHostScanPkgManifestCmd = &cobra.Command{
		Use:     "scan-pkg-manifest <manifest>",
		Aliases: []string{"manifest"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "request an on-demand host vulnerability assessment from a package-manifest",
		Long: `Request an on-demand host vulnerability assessment of your software packages to
determine if the packages contain any common vulnerabilities and exposures.

Simple usage:

    $ lacework vulnerability host scan-pkg-manifest '{
        "os_pkg_info_list": [
            {
                "os":"Ubuntu",
                "os_ver":"18.04",
                "pkg": "openssl",
                "pkg_ver": "1.1.1-1ubuntu2.1~18.04.5"
            }
        ]
    }'

(*) NOTE:
 - Only packages managed by a package manager for supported OS's are reported.
 - Calls to this operation are rate limited to 10 calls per hour, per access key.
 - This operation is limited to 1k of packages per payload. If you require a payload
   larger than 1k, you must make multiple requests.`,
		RunE: func(_ *cobra.Command, args []string) error {
			response, err := cli.LwApi.Vulnerabilities.Host.Scan(args[0])
			if err != nil {
				return errors.Wrap(err, "unable to request an on-demand host vulnerability scan")
			}

			// TODO @afiune add human readable output
			return cli.OutputJSON(response)
		},
	}

	vulHostListCvesCmd = &cobra.Command{
		Use:   "list-cves",
		Args:  cobra.NoArgs,
		Short: "list the CVEs found in the hosts of your environment",
		Long:  "List the CVEs found in the hosts of your environment.",
		RunE: func(_ *cobra.Command, args []string) error {
			response, err := cli.LwApi.Vulnerabilities.Host.ListCves()
			if err != nil {
				return errors.Wrap(err, "unable to get CVEs from hosts")
			}

			if cli.JSONOutput() {
				return cli.OutputJSON(response.CVEs)
			}

			if len(response.CVEs) == 0 {
				// @afiune add a helpful message, possible things are:
				// 1) host vuln feature is not enabled on the account
				// 2) user doesn't have agents deployed
				// 3) there are actually NO vulnerabilities on any host
				cli.OutputHuman("There are no CVEs in your account.\n")
				return nil
			}

			cli.OutputHuman(hostVulnCVEsToTable(response.CVEs))
			return nil
		},
	}

	vulHostListHostsCmd = &cobra.Command{
		Use:   "list-hosts <cve_id>",
		Args:  cobra.ExactArgs(1),
		Short: "list the hosts with a common CVE id in your environment",
		Long: `List the hosts with a common CVE id in your environment.

To list the CVEs found in the hosts of your environment run:

    $ lacework vulnerability host list-cves`,
		RunE: func(_ *cobra.Command, args []string) error {
			response, err := cli.LwApi.Vulnerabilities.Host.ListHostsWithCVE(args[0])
			if err != nil {
				return errors.Wrap(err, "unable to get hosts with CVE "+args[0])
			}

			if cli.JSONOutput() {
				return cli.OutputJSON(response.Hosts)
			}

			if len(response.Hosts) == 0 {
				// @afiune add a helpful message, possible things are:
				// 1) host vuln feature is not enabled on the account
				// 2) user doesn't have agents deployed
				// 3) there are actually NO vulnerabilities on any host
				cli.OutputHuman("There are no hosts in your account with CVE id '%s'\n", args[0])
				return nil
			}

			cli.OutputHuman(hostVulnHostsToTable(response.Hosts))
			return nil
		},
	}

	vulHostShowAssessmentCmd = &cobra.Command{
		Use:     "show-assessment <machine_id>",
		Aliases: []string{"show"},
		Args:    cobra.ExactArgs(1),
		Short:   "show results of a host vulnerability assessment",
		Long: `Show results of a host vulnerability assessment.

To find the machine id from hosts in your environment, use the command:

    $ lacework vulnerability host list-cves

Grab a CVE id and feed it to the command:

    $ lacework vulnerability host list-hosts my_cve_id`,
		RunE: func(_ *cobra.Command, args []string) error {
			response, err := cli.LwApi.Vulnerabilities.Host.GetHostAssessment(args[0])
			if err != nil {
				return errors.Wrap(err, "unable to get host assessment with id "+args[0])
			}

			if cli.JSONOutput() {
				return cli.OutputJSON(response.Assessment)
			}

			cli.OutputHuman(hostVulnHostDetailsToTable(response.Assessment))
			cli.OutputHuman("\n")
			cli.OutputHuman(hostVulnHostAssessmentCVEsToTable(response.Assessment))
			return nil
		},
	}

	// @afiune this is not yet supported since there is no external API available
	vulHostListAssessmentsCmd = &cobra.Command{
		Use:     "list-assessments",
		Hidden:  true,
		Aliases: []string{"list", "ls"},
		Short:   "list host vulnerability assessments from a time range",
		Long:    "List host vulnerability assessments from a time range.",
		RunE: func(_ *cobra.Command, args []string) error {
			return nil
		},
	}
)

func init() {
	// add sub-commands to the 'vulnerability host' command
	vulHostCmd.AddCommand(vulHostScanPkgManifestCmd)
	vulHostCmd.AddCommand(vulHostListAssessmentsCmd)
	vulHostCmd.AddCommand(vulHostListCvesCmd)
	vulHostCmd.AddCommand(vulHostListHostsCmd)
	vulHostCmd.AddCommand(vulHostShowAssessmentCmd)
}

func hostVulnHostsToTable(hosts []api.HostVulnDetail) string {
	var (
		tableBuilder = &strings.Builder{}
		t            = tablewriter.NewWriter(tableBuilder)
	)

	t.SetHeader([]string{
		"Machine ID",
		"Hostname",
		"External IP",
		"Os",
		"Arch",
		"Provider",
		"Instance ID",
		"AMI",
		"Status",
	})
	t.SetBorder(false)
	t.AppendBulk(hostVulnHostsTable(hosts))
	t.Render()

	return tableBuilder.String()
}

func hostVulnHostsTable(hosts []api.HostVulnDetail) [][]string {
	out := [][]string{}
	for _, host := range hosts {
		out = append(out, []string{
			host.Details.MachineID,
			host.Details.Hostname,
			host.Details.Tags.ExternalIP,
			host.Details.Tags.Os,
			host.Details.Tags.Arch,
			host.Details.Tags.VmProvider,
			host.Details.Tags.InstanceID,
			host.Details.Tags.AmiID,
			host.Details.MachineStatus,
		})
	}

	return out
}

func hostVulnCVEsToTable(cves []api.HostVulnCVE) string {
	var (
		tableBuilder = &strings.Builder{}
		t            = tablewriter.NewWriter(tableBuilder)
	)

	t.SetHeader([]string{
		"CVE",
		"Severity",
		"Package",
		"Pkg Version",
		"Score",
		"OS Version",
		"Hosts",
		"Status",
	})
	t.SetBorder(false)
	t.AppendBulk(hostVulnCVEsTable(cves))
	t.Render()

	return tableBuilder.String()
}

func hostVulnCVEsTable(cves []api.HostVulnCVE) [][]string {
	out := [][]string{}
	for _, cve := range cves {
		for _, pkg := range cve.Packages {
			out = append(out, []string{
				cve.ID,
				pkg.Severity,
				pkg.Name,
				pkg.Version,
				pkg.CvssScore,
				pkg.Namespace,
				pkg.HostCount,
				pkg.Status,
			})
		}
	}

	// order by severity
	sort.Slice(out, func(i, j int) bool {
		return severityOrder(out[i][1]) < severityOrder(out[j][1])
	})

	return out
}

func hostVulnHostDetailsToTable(assessment api.HostVulnHostAssessment) string {
	var (
		tableBuilder = &strings.Builder{}
		t            = tablewriter.NewWriter(tableBuilder)
	)

	t.SetHeader([]string{
		"Machine ID",
		"Hostname",
		"External IP",
		"Os",
		"Arch",
		"Provider",
		"Instance ID",
		"AMI",
		"Status",
	})
	t.SetBorder(false)
	t.Append(
		[]string{
			assessment.Host.MachineID,
			assessment.Host.Hostname,
			assessment.Host.Tags.ExternalIP,
			assessment.Host.Tags.Os,
			assessment.Host.Tags.Arch,
			assessment.Host.Tags.VmProvider,
			assessment.Host.Tags.InstanceID,
			assessment.Host.Tags.AmiID,
			assessment.Host.MachineStatus,
		},
	)
	t.Render()

	return tableBuilder.String()
}

func hostVulnHostAssessmentCVEsToTable(assessment api.HostVulnHostAssessment) string {
	var (
		tableBuilder = &strings.Builder{}
		t            = tablewriter.NewWriter(tableBuilder)
	)

	t.SetHeader([]string{
		"CVE",
		"Severity",
		"Score",
		"Package",
		"Pkg Version",
		"Fix Version",
		"Status",
	})
	t.SetBorder(false)
	t.AppendBulk(hostVulnCVEsTableForHostView(assessment.CVEs))
	t.Render()

	return tableBuilder.String()
}

func hostVulnCVEsTableForHostView(cves []api.HostVulnCVE) [][]string {
	out := [][]string{}
	for _, cve := range cves {
		for _, pkg := range cve.Packages {
			out = append(out, []string{
				cve.ID,
				pkg.Severity,
				pkg.CvssScore,
				pkg.Name,
				pkg.Version,
				pkg.FixedVersion,
				pkg.Status,
			})
		}
	}

	// order by severity
	sort.Slice(out, func(i, j int) bool {
		return severityOrder(out[i][1]) < severityOrder(out[j][1])
	})

	return out
}

// @afiune maybe a flag --summary ???
//func _hostVulnCVEsToTableSummary(cves []api.HostVulnCVE) string {
//var (
//tableBuilder = &strings.Builder{}
//t            = tablewriter.NewWriter(tableBuilder)
//)

//t.SetHeader([]string{
//"CVE ID",
//"Severity",
//"Vuln Count",
//"Packages",
//})
//t.SetBorder(false)
//t.AppendBulk(hostVulnCVEsTable(cves))
//t.Render()

//return tableBuilder.String()
//}

//func _hostVulnCVEsTableSummary(cves []api.HostVulnCVE) [][]string {
//out := [][]string{}
//for _, cve := range cves {
//severity := ""
//pkgs := []string{}

//for _, pkg := range cve.Packages {
//if severityOrder(severity) > severityOrder(pkg.Severity) {
//severity = pkg.Severity
//}
//// TODO @afiune constant or variable to customize
//if len(pkgs) < 11 {
//pkgs = append(pkgs, pkg.Name)
//}
//}

//out = append(out, []string{
//cve.ID,
//severity,
//fmt.Sprintf("%d", cve.Summary.TotalVulnerabilities),
//strings.Join(pkgs, ","),
//})
//}

//// order by severity
//sort.Slice(out, func(i, j int) bool {
//return severityOrder(out[i][1]) < severityOrder(out[j][1])
//})

//return out
//}
