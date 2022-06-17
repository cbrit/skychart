package types

type Path struct {
	Chain1 struct {
		ChainName    string `json:"chain-name"`
		ClientID     string `json:"client-id"`
		ConnectionID string `json:"connection-id"`
	} `json:"chain-1"`
	Chain2 struct {
		ChainName    string `json:"chain-name"`
		ClientID     string `json:"client-id"`
		ConnectionID string `json:"connection-id"`
	} `json:"chain-2"`
	Channels []struct {
		Chain1 struct {
			ChannelID string `json:"channel-id"`
			PortID    string `json:"port-id"`
		} `json:"chain-1"`
		Chain2 struct {
			ChannelID string `json:"channel-id"`
			PortID    string `json:"port-id"`
		} `json:"chain-2"`
		Ordering string `json:"ordering"`
		Version  string `json:"version"`
		Tags     struct {
			Status     string `json:"status"`
			Preferred  bool   `json:"preferred"`
			Dex        string `json:"dex"`
			Properties string `json:"properties"`
		} `json:"tags"`
	} `json:"channels"`
}
