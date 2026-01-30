package ami

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"apicall/internal/config"
)

// Client representa un cliente AMI
type Client struct {
	config    *config.AMIConfig
	conn      net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer
	mu        sync.Mutex
	connected bool
	subscribers []chan Event // List of subscribers
	done      chan struct{}
}

// Event representa un evento AMI
type Event struct {
	Type   string
	Fields map[string]string
}

// NewClient crea un nuevo cliente AMI
func NewClient(cfg *config.AMIConfig) *Client {
	return &Client{
		config:      cfg,
		subscribers: make([]chan Event, 0),
		done:        make(chan struct{}),
	}
}

// Connect establece conexión con el AMI
func (c *Client) Connect() error {
	addr := c.config.Address()
	log.Printf("[AMI] Conectando a %s", addr)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("error conectando: %w", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)

	// Leer banner inicial
	if _, err := c.reader.ReadString('\n'); err != nil {
		return fmt.Errorf("error leyendo banner: %w", err)
	}

	// Autenticar
	if err := c.login(); err != nil {
		c.conn.Close()
		return err
	}

	c.connected = true
	log.Printf("[AMI] Conectado correctamente")

	// Iniciar goroutine para procesar eventos
	go c.readEvents()

	return nil
}

// login autentica con el servidor AMI
func (c *Client) login() error {
	action := fmt.Sprintf("Action: Login\r\nUsername: %s\r\nSecret: %s\r\n\r\n",
		c.config.Username, c.config.Secret)

	if _, err := c.writer.WriteString(action); err != nil {
		return err
	}
	if err := c.writer.Flush(); err != nil {
		return err
	}

	// Leer respuesta
	response, err := c.readResponse()
	if err != nil {
		return err
	}

	if response.Fields["Response"] != "Success" {
		return fmt.Errorf("login fallido: %s", response.Fields["Message"])
	}

	return nil
}

// readResponse lee una respuesta completa del AMI
func (c *Client) readResponse() (*Event, error) {
	fields := make(map[string]string)

	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		// Parsear "Key: Value"
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			fields[parts[0]] = parts[1]
		}
	}

	return &Event{
		Type:   fields["Event"],
		Fields: fields,
	}, nil
}

// readEvents lee eventos continuamente
func (c *Client) readEvents() {
	// No cerrar c.events aquí con defer, ya que este método puede reiniciarse
	// múltiples veces durante la vida del cliente (en reconexiones).
	// El canal se cerrará implícitamente/GC o en Close() si se implementa.

	for {
		select {
		case <-c.done:
			return
		default:
			event, err := c.readResponse()
			if err != nil {
				log.Printf("[AMI] Error leyendo evento: %v", err)
				c.reconnect() // Bloquea hasta reconectar
				return        // Terminar esta goroutine, Connect() ya lanzó una nueva
			}

			// Broadcast to all subscribers
			c.mu.Lock()
			for _, sub := range c.subscribers {
				select {
				case sub <- *event:
				default:
					// Subscriber buffer full, drop event for this subscriber
				}
			}
			c.mu.Unlock()
		}
	}
}

// Subscribe returns a channel that receives all AMI events
func (c *Client) Subscribe() <-chan Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Buffered channel for the subscriber
	ch := make(chan Event, 2000)
	c.subscribers = append(c.subscribers, ch)
	return ch
}

// reconnect intenta reconectar al AMI
func (c *Client) reconnect() {
	c.mu.Lock()
	c.connected = false
	if c.conn != nil {
		c.conn.Close()
	}
	c.mu.Unlock()

	for {
		// Verificar si debemos detenernos
		select {
		case <-c.done:
			return
		default:
		}

		log.Printf("[AMI] Reconectando en %d segundos...", c.config.ReconnectInterval)
		time.Sleep(time.Duration(c.config.ReconnectInterval) * time.Second)

		if err := c.Connect(); err != nil {
			log.Printf("[AMI] Error reconectando: %v", err)
		} else {
			// Conexión exitosa, Connect() ya inició una nueva readEvents goroutine
			return
		}
	}
}

// sendAction envía una acción al AMI
func (c *Client) sendAction(action string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("no conectado al AMI")
	}

	if _, err := c.writer.WriteString(action); err != nil {
		return err
	}
	return c.writer.Flush()
}

// SendAction is the public version of sendAction for external use
func (c *Client) SendAction(action string) error {
	return c.sendAction(action)
}

// Close cierra la conexión AMI
func (c *Client) Close() error {
	close(c.done)
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Events is deprecated in favor of Subscribe. 
// Kept temporarily for backward compatibility if needed, 
// but in this refactor we should use Subscribe.
func (c *Client) Events() <-chan Event {
	// For backward compatibility, create a subscription if called
	// but mostly we should update callers.
	return c.Subscribe()
}
