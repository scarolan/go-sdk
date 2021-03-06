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
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/lacework/go-sdk/api"
	"github.com/lacework/go-sdk/internal/array"
)

var (
	eventsCmdState = struct {
		// start time for listing events
		Start string

		// end time for listing events
		End string

		// list events from an specific number of days
		Days int

		// list events with a specific severity
		Severity string
	}{}

	// easily add or remove borders to all event details tables
	eventDetailsBorder = true

	// eventCmd represents the event command
	eventCmd = &cobra.Command{
		Use:     "event",
		Aliases: []string{"events"},
		Short:   "inspect Lacework events",
		Long:    `Inspect events reported by the Lacework platform`,
	}

	// eventListCmd represents the list sub-command inside the event command
	eventListCmd = &cobra.Command{
		Use:   "list",
		Short: "list all events (default last 7 days)",
		Long: `List all events for the last 7 days by default, or pass --start and --end to
specify a custom time period. You can also pass --serverity to filter by a
severity threshold.

Additionally, pass --days to list events for a specified number of days.

For example, to list all events from the last day with severity medium and above
(Critical, High and Medium) run:

    $ lacework events list --severity medium --days 1`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {

			var (
				response api.EventsResponse
				err      error
			)

			if eventsCmdState.Severity != "" {
				if !array.ContainsStr(api.ValidEventSeverities, eventsCmdState.Severity) {
					return errors.Errorf("the severity %s is not valid, use one of %s",
						eventsCmdState.Severity, strings.Join(api.ValidEventSeverities, ", "),
					)
				}
			}

			if eventsCmdState.Start != "" || eventsCmdState.End != "" {
				start, end, errT := parseStartAndEndTime(eventsCmdState.Start, eventsCmdState.End)
				if errT != nil {
					return errors.Wrap(errT, "unable to parse time range")
				}

				cli.Log.Infow("requesting list of events from custom time range",
					"start_time", start, "end_time", end,
				)
				response, err = cli.LwApi.Events.ListDateRange(start, end)
			} else if eventsCmdState.Days != 0 {
				end := time.Now()
				start := end.Add(time.Hour * 24 * time.Duration(eventsCmdState.Days) * -1)

				cli.Log.Infow("requesting list of events from specific days",
					"days", eventsCmdState.Days, "start_time", start, "end_time", end,
				)
				response, err = cli.LwApi.Events.ListDateRange(start, end)
			} else {
				cli.Log.Info("requesting list of events from the last 7 days")
				response, err = cli.LwApi.Events.List()
			}

			if err != nil {
				return errors.Wrap(err, "unable to get events")
			}

			cli.Log.Debugw("events", "raw", response)

			// filter events by severity, if the user didn't specify a severity
			// the funtion will return it back without modifications
			events := filterEventsWithSeverity(response.Events)

			// Sort the events by severity
			sort.Slice(events, func(i, j int) bool {
				return events[i].Severity < events[j].Severity
			})

			if cli.JSONOutput() {
				return cli.OutputJSON(events)
			}

			if len(events) == 0 {
				if eventsCmdState.Severity != "" {
					cli.OutputHuman("There are no events with the specified severity.\n")
				} else {
					cli.OutputHuman("There are no events in your account in the specified time range.\n")
				}
				return nil
			}

			cli.OutputHuman(eventsToTableReport(events))
			return nil
		},
	}

	// eventShowCmd represents the show sub-command inside the event command
	eventShowCmd = &cobra.Command{
		Use:   "show <event_id>",
		Short: "show details about a specific event",
		Long:  "Show details about a specific event.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cli.Log.Infow("requesting event details", "event_id", args[0])
			response, err := cli.LwApi.Events.Details(args[0])
			if err != nil {
				return errors.Wrap(err, "unable to get event details")
			}

			cli.Log.Debugw("event details",
				"event_id", args[0],
				"raw", response,
			)
			if len(response.Events) == 0 {
				return errors.Errorf("there are no details about the event '%s'", args[0])
			}

			// @afiune why do we have an array of events when we ask for details
			// about a single event? Let us use the first one for now
			if cli.JSONOutput() {
				return cli.OutputJSON(response.Events[0])
			}

			cli.OutputHuman(eventDetailsSummaryReport(response.Events[0]))
			for _, entityTable := range eventEntityMapTables(response.Events[0].EntityMap) {
				cli.OutputHuman("\n")
				cli.OutputHuman(entityTable)
			}

			cli.OutputHuman(
				"\nFor further investigation of this event navigate to %s\n",
				eventLinkBuilder(args[0]),
			)
			return nil
		},
	}

	// eventOpenCmd represents the open sub-command inside the event command
	eventOpenCmd = &cobra.Command{
		Use:   "open <event_id>",
		Short: "open a specified event in a web browser",
		Long:  "Open a specified event in a web browser.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			// Event IDs should be only numeric values
			if _, err := strconv.Atoi(args[0]); err != nil {
				return errors.Errorf("invalid event id %s. Event id should be a numeric value", args[0])
			}

			var (
				err error
				url = eventLinkBuilder(args[0])
			)

			switch runtime.GOOS {
			case "linux":
				err = exec.Command("xdg-open", url).Start()
			case "windows":
				err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
			case "darwin":
				err = exec.Command("open", url).Start()
			default:
				err = fmt.Errorf("unsupported platform\n\nNavigate to %s", url)
			}
			if err != nil {
				return errors.Wrap(err, "unable to open web browser")
			}

			return nil
		},
	}
)

func init() {
	// add the event command
	rootCmd.AddCommand(eventCmd)

	// add sub-commands to the event command
	eventCmd.AddCommand(eventListCmd)

	// add start flag to events list command
	eventListCmd.Flags().StringVar(&eventsCmdState.Start,
		"start", "", "start of the time range in UTC (format: yyyy-MM-ddTHH:mm:ssZ)",
	)
	// add end flag to events list command
	eventListCmd.Flags().StringVar(&eventsCmdState.End,
		"end", "", "end of the time range in UTC (format: yyyy-MM-ddTHH:mm:ssZ)",
	)
	// add days flag to events list command
	eventListCmd.Flags().IntVar(&eventsCmdState.Days,
		"days", 0, "list events for specified number of days (max: 7 days)",
	)
	// add severity flag to events list command
	eventListCmd.Flags().StringVar(&eventsCmdState.Severity,
		"severity", "",
		fmt.Sprintf(
			"filter events by severity threshold (%s)",
			strings.Join(api.ValidEventSeverities, ", "),
		),
	)

	eventCmd.AddCommand(eventShowCmd)
	eventCmd.AddCommand(eventOpenCmd)
}

// Generates a URL similar to:
//   => https://account.lacework.net/ui/investigate/recents/EventDossier-123
func eventLinkBuilder(id string) string {
	return fmt.Sprintf("https://%s.lacework.net/ui/investigation/recents/EventDossier-%s", cli.Account, id)
}

func eventsToTableReport(events []api.Event) string {
	var (
		eventsReport = &strings.Builder{}
		t            = tablewriter.NewWriter(eventsReport)
	)

	t.SetHeader([]string{
		"Event ID",
		"Type",
		"Severity",
		"Start Time",
		"End Time",
	})
	t.SetBorder(false)
	t.AppendBulk(eventsToTable(events))
	t.Render()

	return eventsReport.String()
}

func eventsToTable(events []api.Event) [][]string {
	out := [][]string{}
	for _, event := range events {
		out = append(out, []string{
			event.EventID,
			event.EventType,
			event.SeverityString(),
			event.StartTime.UTC().Format(time.RFC3339),
			event.EndTime.UTC().Format(time.RFC3339),
		})
	}
	return out
}

func eventDetailsSummaryReport(details api.EventDetails) string {
	var (
		report = &strings.Builder{}
		t      = tablewriter.NewWriter(report)
	)

	t.SetHeader([]string{
		"Event ID",
		"Type",
		"Actor",
		"Model",
		"Start Time",
		"End Time",
	})
	t.SetBorder(eventDetailsBorder)
	t.Append([]string{
		details.EventID,
		details.EventType,
		details.EventActor,
		details.EventModel,
		details.StartTime.UTC().Format(time.RFC3339),
		details.EndTime.UTC().Format(time.RFC3339),
	})
	t.Render()

	return report.String()
}

func eventEntityMapTables(eventEntities api.EventEntityMap) []string {
	tables := []string{}

	if machineTable := eventMachineEntitiesTable(eventEntities.Machine); machineTable != "" {
		tables = append(tables, machineTable)
	}
	if containerTable := eventContainerEntitiesTable(eventEntities.Container); containerTable != "" {
		tables = append(tables, containerTable)
	}
	if appTable := eventApplicationEntitiesTable(eventEntities.Application); appTable != "" {
		tables = append(tables, appTable)
	}
	if userTable := eventUserEntitiesTable(eventEntities.User); userTable != "" {
		tables = append(tables, userTable)
	}
	if ipaddressTable := eventIpAddressEntitiesTable(eventEntities.IpAddress); ipaddressTable != "" {
		tables = append(tables, ipaddressTable)
	}
	if sourceIpAddrTable := eventSourceIpAddressEntitiesTable(eventEntities.SourceIpAddress); sourceIpAddrTable != "" {
		tables = append(tables, sourceIpAddrTable)
	}
	if dnsTable := eventDnsNameEntitiesTable(eventEntities.DnsName); dnsTable != "" {
		tables = append(tables, dnsTable)
	}
	if apiTable := eventAPIEntitiesTable(eventEntities.API); apiTable != "" {
		tables = append(tables, apiTable)
	}
	if ctUserTable := eventCTUserEntitiesTable(eventEntities.CTUser); ctUserTable != "" {
		tables = append(tables, ctUserTable)
	}
	if regionTable := eventRegionEntitiesTable(eventEntities.Region); regionTable != "" {
		tables = append(tables, regionTable)
	}
	if processTable := eventProcessEntitiesTable(eventEntities.Process); processTable != "" {
		tables = append(tables, processTable)
	}
	if exePathTable := eventFileExePathEntitiesTable(eventEntities.FileExePath); exePathTable != "" {
		tables = append(tables, exePathTable)
	}
	if dataHashTable := eventFileDataHashEntitiesTable(eventEntities.FileDataHash); dataHashTable != "" {
		tables = append(tables, dataHashTable)
	}
	if cRuleTable := eventCustomRuleEntitiesTable(eventEntities.CustomRule); cRuleTable != "" {
		tables = append(tables, cRuleTable)
	}
	if violationTable := eventNewViolationEntitiesTable(eventEntities.NewViolation); violationTable != "" {
		tables = append(tables, violationTable)
	}
	if recordsTable := eventRecIDEntitiesTable(eventEntities.RecID); recordsTable != "" {
		tables = append(tables, recordsTable)
	}
	if vReasonTable := eventViolationReasonEntitiesTable(eventEntities.ViolationReason); vReasonTable != "" {
		tables = append(tables, vReasonTable)
	}
	if resourceTable := eventResourceEntitiesTable(eventEntities.Resource); resourceTable != "" {
		tables = append(tables, resourceTable)
	}

	return tables
}

func eventRegionEntitiesTable(regions []api.EventRegionEntity) string {
	if len(regions) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Region",
		"Accounts",
	})
	t.SetBorder(eventDetailsBorder)
	for _, user := range regions {
		t.Append([]string{
			user.Region,
			strings.Join(user.AccountList, ", "),
		})
	}
	t.Render()

	return r.String()
}

func eventCTUserEntitiesTable(users []api.EventCTUserEntity) string {
	if len(users) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Username",
		"Account ID",
		"Principal ID",
		"MFA",
		"List of APIs",
		"Regions",
	})
	t.SetBorder(eventDetailsBorder)
	for _, user := range users {
		mfa := "Disabled"
		if user.Mfa != 0 {
			mfa = "Enabled"
		}
		t.Append([]string{
			user.Username,
			user.AccountID,
			user.PrincipalID,
			mfa,
			strings.Join(user.ApiList, ", "),
			strings.Join(user.RegionList, ", "),
		})
	}
	t.Render()

	return r.String()
}

func eventDnsNameEntitiesTable(dnss []api.EventDnsNameEntity) string {
	if len(dnss) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"DNS Hostname",
		"List of Ports",
		"Inbound Bytes",
		"Outboud Bytes",
	})
	t.SetBorder(eventDetailsBorder)
	for _, d := range dnss {
		t.Append([]string{
			d.Hostname,
			array.JoinInt32(d.PortList, ", "),
			fmt.Sprintf("%.3f", d.TotalInBytes),
			fmt.Sprintf("%.3f", d.TotalOutBytes),
		})
	}
	t.Render()

	return r.String()
}

func eventAPIEntitiesTable(apis []api.EventAPIEntity) string {
	if len(apis) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Service",
		"API",
	})
	t.SetBorder(eventDetailsBorder)
	for _, a := range apis {
		t.Append([]string{
			a.Service,
			a.Api,
		})
	}
	t.Render()

	return r.String()
}

func eventSourceIpAddressEntitiesTable(ips []api.EventSourceIpAddressEntity) string {
	if len(ips) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Source IP Address",
		"Country",
		"Region",
	})
	t.SetBorder(eventDetailsBorder)
	for _, ip := range ips {
		t.Append([]string{
			ip.IpAddress,
			ip.Country,
			ip.Region,
		})
	}
	t.Render()

	return r.String()
}

func eventIpAddressEntitiesTable(ips []api.EventIpAddressEntity) string {
	if len(ips) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"IP Address",
		"Inbound Bytes",
		"Outboud Bytes",
		"List of Ports",
		"First Time Seen",
		"Threat Tags",
		"Threat Source",
		"Country",
		"Region",
	})
	t.SetBorder(eventDetailsBorder)
	for _, ip := range ips {
		t.Append([]string{
			ip.IpAddress,
			fmt.Sprintf("%.3f", ip.TotalInBytes),
			fmt.Sprintf("%.3f", ip.TotalOutBytes),
			array.JoinInt32(ip.PortList, ", "),
			ip.FirstSeenTime.UTC().Format(time.RFC3339),
			ip.ThreatTags,
			fmt.Sprintf("%v", ip.ThreatSource),
			ip.Country,
			ip.Region,
		})
	}
	t.Render()

	return r.String()
}

func eventFileDataHashEntitiesTable(dataHashes []api.EventFileDataHashEntity) string {
	if len(dataHashes) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Executable Paths",
		"File Hash",
		"Number of Machines",
		"First Time Seen",
		"Known Bad",
	})
	t.SetBorder(eventDetailsBorder)
	for _, dHash := range dataHashes {
		knownBad := "No"
		if dHash.IsKnownBad != 0 {
			knownBad = "Yes"
		}
		t.Append([]string{
			strings.Join(dHash.ExePathList, ", "),
			dHash.FiledataHash,
			fmt.Sprintf("%d", dHash.MachineCount),
			dHash.FirstSeenTime.UTC().Format(time.RFC3339),
			knownBad,
		})
	}
	t.Render()

	return r.String()
}

func eventFileExePathEntitiesTable(exePaths []api.EventFileExePathEntity) string {
	if len(exePaths) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Executable Path",
		"First Time Seen",
		"Last File Hash",
		"Last Package Name",
		"Last Version",
		"Last File Owner",
	})
	t.SetBorder(eventDetailsBorder)
	for _, exe := range exePaths {
		t.Append([]string{
			exe.ExePath,
			exe.FirstSeenTime.UTC().Format(time.RFC3339),
			exe.LastFiledataHash,
			exe.LastPackageName,
			exe.LastVersion,
			exe.LastFileOwner,
		})
	}
	t.Render()

	return r.String()
}

func eventProcessEntitiesTable(processes []api.EventProcessEntity) string {
	if len(processes) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Process ID",
		"Hostname",
		"Start Time",
		"CPU Percentage",
		"Command",
	})
	t.SetBorder(eventDetailsBorder)
	for _, proc := range processes {
		t.Append([]string{
			fmt.Sprintf("%d", proc.ProcessID),
			proc.Hostname,
			proc.ProcessStartTime.UTC().Format(time.RFC3339),
			fmt.Sprintf("%.3f", proc.CpuPercentage),
			proc.Cmdline,
		})
	}
	t.Render()

	return r.String()
}

func eventContainerEntitiesTable(containers []api.EventContainerEntity) string {
	if len(containers) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Image Repo",
		"Image Tag",
		"External Connections",
		"Type",
		"First Time Seen",
		"Pod Namespace",
		"Pod Ipaddress",
	})
	t.SetBorder(eventDetailsBorder)
	for _, container := range containers {
		containerType := ""
		if container.IsClient != 0 {
			containerType = "Client"
		}
		if container.IsServer != 0 {
			if containerType != "" {
				containerType = "Server/Client"
			} else {
				containerType = "Server"
			}
		}
		t.Append([]string{
			container.ImageRepo,
			container.ImageTag,
			fmt.Sprintf("%d", container.HasExternalConns),
			containerType,
			container.FirstSeenTime.UTC().Format(time.RFC3339),
			container.PodNamespace,
			container.PodIpAddr,
		})
	}
	t.Render()

	return r.String()
}

func eventUserEntitiesTable(users []api.EventUserEntity) string {
	if len(users) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Username",
		"Hostname",
	})
	t.SetBorder(eventDetailsBorder)
	for _, user := range users {
		t.Append([]string{
			user.Username,
			user.MachineHostname,
		})
	}
	t.Render()

	return r.String()
}

func eventApplicationEntitiesTable(applications []api.EventApplicationEntity) string {
	if len(applications) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Application",
		"External Connections",
		"Type",
		"Earliest Known Time",
	})
	t.SetBorder(eventDetailsBorder)
	for _, app := range applications {
		appType := ""
		if app.IsClient != 0 {
			appType = "Client"
		}
		if app.IsServer != 0 {
			if appType != "" {
				appType = "Server/Client"
			} else {
				appType = "Server"
			}
		}
		t.Append([]string{
			app.Application,
			fmt.Sprintf("%d", app.HasExternalConns),
			appType,
			app.EarliestKnownTime.UTC().Format(time.RFC3339),
		})
	}
	t.Render()

	return r.String()
}

func eventCustomRuleEntitiesTable(rules []api.EventCustomRuleEntity) string {
	if len(rules) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)
	t.SetBorder(false)
	t.SetAutoWrapText(false)

	for _, rule := range rules {
		t.Append([]string{eventCustomRuleEntityTable(rule)})
		t.Append([]string{eventCustomRuleDisplayFilerTable(rule)})
	}
	t.Render()

	return r.String()
}

func eventCustomRuleEntityTable(rule api.EventCustomRuleEntity) string {
	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)
	t.SetHeader([]string{
		"Rule GUID",
		"Last Updated User",
		"Last Updated Time",
	})
	t.SetBorder(eventDetailsBorder)
	t.SetAutoWrapText(false)
	t.Append([]string{
		rule.RuleGuid,
		rule.LastUpdatedUser,
		rule.LastUpdatedTime.UTC().Format(time.RFC3339),
	})
	t.Render()
	return r.String()
}

func eventCustomRuleDisplayFilerTable(rule api.EventCustomRuleEntity) string {
	filter, err := cli.FormatJSONString(rule.DisplayFilter)
	if err != nil {
		filter = rule.DisplayFilter
	}

	return oneLineTable("Display Filter", filter)
}

func oneLineTable(title, content string) string {
	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{title})
	t.SetBorder(eventDetailsBorder)
	t.SetAutoWrapText(false)
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	t.Append([]string{content})
	t.Render()

	return r.String()
}

func eventRecIDEntitiesTable(records []api.EventRecIDEntity) string {
	if len(records) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Record ID",
		"Account ID",
		"Account Alias",
		"Description",
		"Status",
		"Evaluation Type",
		"Evaluation GUID",
	})
	t.SetBorder(eventDetailsBorder)
	for _, rec := range records {
		t.Append([]string{
			rec.RecID,
			rec.AccountID,
			rec.AccountAlias,
			rec.Title,
			rec.Status,
			rec.EvalType,
			rec.EvalGuid,
		})
	}
	t.Render()

	return r.String()
}

func eventViolationReasonEntitiesTable(reasons []api.EventViolationReasonEntity) string {
	if len(reasons) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Violation ID",
		"Reason",
	})
	t.SetBorder(eventDetailsBorder)
	for _, reason := range reasons {
		t.Append([]string{
			reason.RecID,
			reason.Reason,
		})
	}
	t.Render()

	return r.String()
}

func eventResourceEntitiesTable(resources []api.EventResourceEntity) string {
	if len(resources) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Name",
		"Value",
	})
	t.SetBorder(eventDetailsBorder)
	for _, res := range resources {
		t.Append([]string{
			res.Name,
			fmt.Sprintf("%v", res.Value),
		})
	}
	t.Render()

	return r.String()
}

func eventNewViolationEntitiesTable(violations []api.EventNewViolationEntity) string {
	if len(violations) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Violation ID",
		"Reason",
		"Resource",
	})
	t.SetBorder(eventDetailsBorder)
	for _, v := range violations {
		t.Append([]string{
			v.RecID,
			v.Reason,
			v.Resource,
		})
	}
	t.Render()

	return r.String()
}

func eventMachineEntitiesTable(machines []api.EventMachineEntity) string {
	if len(machines) == 0 {
		return ""
	}

	var (
		r = &strings.Builder{}
		t = tablewriter.NewWriter(r)
	)

	t.SetHeader([]string{
		"Hostname",
		"External IP",
		"Instance ID",
		"Instance Name",
		"CPU Percentage",
		"Internal Ipaddress",
	})
	t.SetBorder(eventDetailsBorder)
	for _, m := range machines {
		t.Append([]string{
			m.Hostname,
			m.ExternalIp,
			m.InstanceID,
			m.InstanceName,
			fmt.Sprintf("%.3f", m.CpuPercentage),
			m.InternalIpAddress,
		})
	}
	t.Render()

	return r.String()
}

func filterEventsWithSeverity(events []api.Event) []api.Event {
	if eventsCmdState.Severity == "" {
		return events
	}

	sevThreshold, sevString := eventSeverityToProperTypes(eventsCmdState.Severity)
	cli.Log.Debugw("filtering events", "threshold", sevThreshold, "severity", sevString)
	eFiltered := []api.Event{}
	for _, event := range events {
		eventSeverity, _ := eventSeverityToProperTypes(event.Severity)
		if eventSeverity <= sevThreshold {
			eFiltered = append(eFiltered, event)
		}
	}

	cli.Log.Debugw("filtered events", "events", eFiltered)

	return eFiltered
}

func eventSeverityToProperTypes(severity string) (int, string) {
	switch strings.ToLower(severity) {
	case "1", "critical":
		return 1, "Critical"
	case "2", "high":
		return 2, "High"
	case "3", "medium":
		return 3, "Medium"
	case "4", "low":
		return 4, "Low"
	case "5", "info":
		return 5, "Info"
	default:
		return 6, "Unknown"
	}
}
