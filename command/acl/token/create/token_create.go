package tokencreate

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string

	policyIDs   []string
	policyNames []string
	description string
	local       bool
	showMeta    bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that token metadata such "+
		"as the content hash and raft indices should be shown for each entry")
	c.flags.BoolVar(&c.local, "local", false, "Create this as a datacenter local token")
	c.flags.StringVar(&c.description, "description", "", "A description of the token")
	c.flags.Var((*flags.AppendSliceValue)(&c.policyIDs), "policy-id", "ID of a "+
		"policy to use for this token. May be specified multiple times")
	c.flags.Var((*flags.AppendSliceValue)(&c.policyNames), "policy-name", "Name of a "+
		"policy to use for this token. May be specified multiple times")
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if len(c.policyNames) == 0 && len(c.policyIDs) == 0 {
		c.UI.Error(fmt.Sprintf("Cannot create a token without specifying -policy-name or -policy-id at least once"))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	newToken := &api.ACLToken{
		Description: c.description,
		Local:       c.local,
	}

	for _, policyName := range c.policyNames {
		// We could resolve names to IDs here but there isn't any reason why its would be better
		// than allowing the agent to do it.
		newToken.Policies = append(newToken.Policies, &api.ACLTokenPolicyLink{Name: policyName})
	}

	for _, policyID := range c.policyIDs {
		policyID, err := acl.GetPolicyIDFromPartial(client, policyID)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error resolving policy ID %s: %v", policyID, err))
			return 1
		}
		newToken.Policies = append(newToken.Policies, &api.ACLTokenPolicyLink{ID: policyID})
	}

	token, _, err := client.ACL().TokenCreate(newToken, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to create new token: %v", err))
		return 1
	}

	acl.PrintToken(token, c.UI, c.showMeta)
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Create an ACL Token"
const help = `
Usage: consul acl token create [options]

  When creating a new token policies may be linked using either the -policy-id
  or the -policy-name options. When specifying policies by IDs you may use a
  unique prefix of the UUID as a shortcut for specifying the entire UUID.

  Create a new token:

          $ consul acl token create -description "Replication token"
                                            -policy-id b52fc3de-5
                                            -policy-name "acl-replication"
`
