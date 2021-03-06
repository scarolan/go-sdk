## lacework compliance gcp

compliance for Google Cloud

### Synopsis

Manage compliance reports for Google Cloud.

To get the latest GCP compliance assessment report, use the command:

    $ lacework compliance gcp get-report <organization_id> <project_id>

These reports run on a regular schedule, typically once a day.

To find out which GCP organizations/projects are connected to your
Lacework account, use the following command:

    $ lacework integrations list --type GCP_CFG

Then, choose one integration, copy the GUID and visualize its details
using the command:

    $ lacework integration show <int_guid>

To list all GCP projects from an organization, use the command:

    $ lacework compliance gcp list-projects <organization_id>

To run an ad-hoc compliance assessment use the command:

    $ lacework compliance gcp run-assessment <org_or_project_id>


### Options

```
  -h, --help   help for gcp
```

### Options inherited from parent commands

```
  -a, --account string      account subdomain of URL (i.e. <ACCOUNT>.lacework.net)
  -k, --api_key string      access key id
  -s, --api_secret string   secret access key
      --debug               turn on debug logging
      --json                switch commands output from human-readable to json format
      --nocolor             turn off colors
      --noninteractive      turn off interactive mode (disable spinners, prompts, etc.)
  -p, --profile string      switch between profiles configured at ~/.lacework.toml
```

### SEE ALSO

* [lacework compliance](lacework_compliance.md)	 - manage compliance reports
* [lacework compliance gcp get-report](lacework_compliance_gcp_get-report.md)	 - get the latest GCP compliance report
* [lacework compliance gcp list-projects](lacework_compliance_gcp_list-projects.md)	 - list projects from an organization
* [lacework compliance gcp run-assessment](lacework_compliance_gcp_run-assessment.md)	 - run a new GCP compliance assessment

