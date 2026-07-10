package trovemodule

import "github.com/hashicorp/go-plugin"

// Handshake is shared between the Trove core and local source modules.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "TROVE_MODULE",
	MagicCookieValue: "trove-source-v1",
}

// PluginName is the go-plugin map key for source modules.
const PluginName = "source"
