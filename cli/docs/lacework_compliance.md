## lacework compliance

manage compliance reports

### Synopsis

Manage compliance reports for GCP, Azure, or AWS cloud providers.

To start sending data about your environment to Lacework for compliance reporting
analysis, configure one or more cloud integration using the following command:

    $ lacework integration create

Or, if you prefer to do it via the WebUI, log in to your account at:

    https://<ACCOUNT>.lacework.net

Then navigate to Settings > Integrations > Cloud Accounts.

Use the following command to list all available integrations in your account:

    $ lacework integrations list


### Options

```
  -h, --help   help for compliance
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

* [lacework](lacework.md)	 - A tool to manage the Lacework cloud security platform.
* [lacework compliance aws](lacework_compliance_aws.md)	 - compliance for AWS
* [lacework compliance azure](lacework_compliance_azure.md)	 - compliance for Microsoft Azure
* [lacework compliance gcp](lacework_compliance_gcp.md)	 - compliance for Google Cloud

