package command

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"regexp"

	"github.com/emersion/go-message/textproto"
	"github.com/foxcpp/maddy/framework/buffer"
	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/log"
	"github.com/foxcpp/maddy/framework/module"
)

const modName = "imap.filter.command"

var placeholderRe = regexp.MustCompile(`{[a-zA-Z0-9_]+?}`)

type Check struct {
	instName string
	log      log.Logger

	cmd     string
	cmdArgs []string
}

func (c *Check) IMAPFilter(accountName string, msgMeta *module.MsgMetadata, hdr textproto.Header, body buffer.Buffer) (folder string, flags []string, err error) {
	cmd, args := c.expandCommand(msgMeta, accountName)

	var buf bytes.Buffer
	_ = textproto.WriteHeader(&buf, hdr)
	bR, err := body.Open()
	if err != nil {
		return "", nil, err
	}

	return c.run(cmd, args, io.MultiReader(bytes.NewReader(buf.Bytes()), bR))
}

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	c := &Check{
		instName: instName,
		log:      log.Logger{Name: modName, Debug: log.DefaultLogger.Debug},
	}

	if len(inlineArgs) == 0 {
		return nil, errors.New("command: at least one argument is required (command name)")
	}

	c.cmd = inlineArgs[0]
	c.cmdArgs = inlineArgs[1:]

	return c, nil
}

func (c *Check) Name() string {
	return modName
}

func (c *Check) InstanceName() string {
	return c.instName
}

func (c *Check) Init(cfg *config.Map) error {
	// Check whether the inline argument command is usable.
	if _, err := exec.LookPath(c.cmd); err != nil {
		return fmt.Errorf("command: %w", err)
	}

	_, err := cfg.Process()
	return err
}

func (c *Check) expandCommand(msgMeta *module.MsgMetadata, accountName string) (string, []string) {
	expArgs := make([]string, len(c.cmdArgs))

	for i, arg := range c.cmdArgs {
		expArgs[i] = placeholderRe.ReplaceAllStringFunc(arg, func(placeholder string) string {
			switch placeholder {
			case "{auth_user}":
				if msgMeta.Conn == nil {
					return ""
				}
				return msgMeta.Conn.AuthUser
			case "{source_ip}":
				if msgMeta.Conn == nil {
					return ""
				}
				tcpAddr, _ := msgMeta.Conn.RemoteAddr.(*net.TCPAddr)
				if tcpAddr == nil {
					return ""
				}
				return tcpAddr.IP.String()
			case "{source_host}":
				if msgMeta.Conn == nil {
					return ""
				}
				return msgMeta.Conn.Hostname
			case "{source_rdns}":
				if msgMeta.Conn == nil {
					return ""
				}
				valI, err := msgMeta.Conn.RDNSName.Get()
				if err != nil {
					return ""
				}
				if valI == nil {
					return ""
				}
				return valI.(string)
			case "{msg_id}":
				return msgMeta.ID
			case "{sender}":
				return msgMeta.OriginalFrom
			case "{account_name}":
				return accountName
			}
			return placeholder
		})
	}

	return c.cmd, expArgs
}

func (c *Check) run(cmdName string, args []string, stdin io.Reader) (string, []string, error) {
	c.log.Debugln("running", cmdName, args)

	cmd := exec.Command(cmdName, args...)
	cmd.Stdin = stdin
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, err
	}

	if err := cmd.Start(); err != nil {
		return "", nil, err
	}

	scnr := bufio.NewScanner(stdout)
	var (
		folder string
		flags  []string
	)
	if scnr.Scan() {
		folder = scnr.Text()
	}
	for scnr.Scan() {
		flags = append(flags, scnr.Text())
	}
	if err := scnr.Err(); err != nil {
		return "", nil, err
	}

	err = cmd.Wait()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			// If that's not ExitError, the process may still be running. We do
			// not want this.
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				c.log.Error("failed to kill process", err)
			}
		}
		return "", nil, err
	}

	c.log.Debugf("folder: %s, extra flags: %v", folder, flags)

	return folder, flags, nil
}

func init() {
	module.Register(modName, New)
}
