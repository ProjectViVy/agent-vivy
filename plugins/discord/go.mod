module agent-vivy/plugins/discord

go 1.26.4

replace agent-vivy => ../..

require (
	agent-vivy v0.0.0
	github.com/bwmarrin/discordgo v0.29.0
	github.com/gorilla/websocket v1.5.3
)

require (
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
