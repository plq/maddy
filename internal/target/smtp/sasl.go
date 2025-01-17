package smtp_downstream

import (
	"github.com/emersion/go-sasl"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/exterrors"
	"github.com/foxcpp/maddy/framework/module"
)

type saslClientFactory = func(msgMeta *module.MsgMetadata) (sasl.Client, error)

// saslAuthDirective returns saslClientFactory function used to create sasl.Client.
// for use in outbound connections.
//
// Authentication information of the current client should be passed in arguments.
func saslAuthDirective(m *config.Map, node config.Node) (interface{}, error) {
	if len(node.Children) != 0 {
		return nil, config.NodeErr(node, "can't declare a block here")
	}
	if len(node.Args) == 0 {
		return nil, config.NodeErr(node, "at least one argument required")
	}
	switch node.Args[0] {
	case "off":
		return nil, nil
	case "forward":
		if len(node.Args) > 1 {
			return nil, config.NodeErr(node, "no additional arguments required")
		}
		return func(msgMeta *module.MsgMetadata) (sasl.Client, error) {
			if msgMeta.Conn == nil || msgMeta.Conn.AuthUser == "" || msgMeta.Conn.AuthPassword == "" {
				return nil, &exterrors.SMTPError{
					Code:         530,
					EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
					Message:      "Authentication is required",
					TargetName:   "target.smtp",
					Reason:       "Credentials forwarding is requested but the client is not authenticated",
				}
			}
			return sasl.NewPlainClient("", msgMeta.Conn.AuthUser, msgMeta.Conn.AuthPassword), nil
		}, nil
	case "plain":
		if len(node.Args) != 3 {
			return nil, config.NodeErr(node, "two additional arguments are required (username, password)")
		}
		return func(*module.MsgMetadata) (sasl.Client, error) {
			return sasl.NewPlainClient("", node.Args[1], node.Args[2]), nil
		}, nil
	case "external":
		if len(node.Args) > 1 {
			return nil, config.NodeErr(node, "no additional arguments required")
		}
		return func(*module.MsgMetadata) (sasl.Client, error) {
			return sasl.NewExternalClient(""), nil
		}, nil
	default:
		return nil, config.NodeErr(node, "unknown authentication mechanism: %s", node.Args[0])
	}
}
