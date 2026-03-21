package ssh

import "fmt"

// DialChain connects through a sequence of hops.
// hops[0] is the first bastion; hops[len-1] is the destination.
// Returns the Client for the final destination.
// Intermediate clients are stored in the returned chain and closed
// when the terminal client is closed via CloseAll.
func DialChain(hops []HopConfig) (*ChainedClient, error) {
	if len(hops) == 0 {
		return nil, fmt.Errorf("no hops provided")
	}

	chain := &ChainedClient{}

	// Dial the first hop directly
	first, err := Dial(hops[0])
	if err != nil {
		return nil, err
	}
	chain.clients = append(chain.clients, first)

	// Each subsequent hop goes through the previous client
	for i := 1; i < len(hops); i++ {
		next, err := DialVia(chain.clients[i-1], hops[i])
		if err != nil {
			chain.CloseAll()
			return nil, fmt.Errorf("hop %d (%s): %w", i, hops[i].Host, err)
		}
		chain.clients = append(chain.clients, next)
	}

	return chain, nil
}

// ChainedClient holds all clients in a multi-hop chain.
// The last element is the terminal destination.
type ChainedClient struct {
	clients []*Client
}

// Terminal returns the SSH client for the final destination.
func (c *ChainedClient) Terminal() *Client {
	if len(c.clients) == 0 {
		return nil
	}
	return c.clients[len(c.clients)-1]
}

// CloseAll closes all clients in the chain in reverse order.
func (c *ChainedClient) CloseAll() {
	for i := len(c.clients) - 1; i >= 0; i-- {
		c.clients[i].Close()
	}
}
