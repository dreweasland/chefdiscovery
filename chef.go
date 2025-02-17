package chef

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/go-chef/chef"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/promslog"

	"github.com/prometheus/common/model"

	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/refresh"
	"github.com/prometheus/prometheus/discovery/targetgroup"
)

const (
	chefLabel                = model.MetaLabelPrefix + "chef_"
	chefLabelNodeID          = chefLabel + "node_id"
	chefLabelNodeURL         = chefLabel + "node_url"
	chefLabelNodeName        = chefLabel + "node_name"
	chefLabelNodeOSType      = chefLabel + "node_os_type"
	chefLabelNodeEnvironment = chefLabel + "node_environment"
	chefLabelNodeIP          = chefLabel + "node_ip"
	chefLabelNodeAttribute   = chefLabel + "node_attribute_"
	chefLabelNodeTag         = chefLabel + "node_tag"
	chefLabelNodeRole        = chefLabel + "node_role"

	namespace = "prometheus"
)

// DefaultSDConfig is the default Chef SD configuration.
var DefaultSDConfig = SDConfig{
	Port:            9090,
	RefreshInterval: model.Duration(5 * time.Minute),
	IgnoreSSL:       false,
	MetaAttribute:   []map[string]interface{}{},
}

func init() {
	discovery.RegisterConfig(&SDConfig{})
}

// SDConfig is the configuration for Chef based service discovery.
type SDConfig struct {
	Port            int                      `yaml:"port"`
	ChefServer      string                   `yaml:"chef_server"`
	UserID          string                   `yaml:"user_id,omitempty"`
	UserKey         config_util.Secret       `yaml:"user_key,omitempty"`
	UserKeyLocation string                   `yaml:"user_key_file,omitempty"`
	RefreshInterval model.Duration           `yaml:"refresh_interval,omitempty"`
	IgnoreSSL       bool                     `yaml:"ignore_ssl,omitempty"`
	MetaAttribute   []map[string]interface{} `yaml:"meta_attribute"`
}

type ChefClient struct {
	*chef.Client
}

// NewDiscovererMetrics implements discovery.Config.
func (*SDConfig) NewDiscovererMetrics(reg prometheus.Registerer, rmi discovery.RefreshMetricsInstantiator) discovery.DiscovererMetrics {
	return newDiscovererMetrics(reg, rmi)
}

// Name returns the name of the Config.
func (*SDConfig) Name() string { return "chef" }

// NewDiscoverer returns a Discoverer for the Config.
func (c *SDConfig) NewDiscoverer(opts discovery.DiscovererOptions) (discovery.Discoverer, error) {
	return NewDiscovery(c, opts.Logger, opts.Metrics)
}

func validateAuthParam(param, name string) error {
	if len(param) == 0 {
		return errors.Errorf("chef SD configuration requires a %s", name)
	}
	return nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *SDConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultSDConfig
	type plain SDConfig
	err := unmarshal((*plain)(c))
	if err != nil {
		return err
	}

	if err = validateAuthParam(c.UserID, "user_id"); err != nil {
		return err
	}

	if err = validateAuthParam(c.ChefServer, "chef_server"); err != nil {
		return err
	}

	if c.UserKeyLocation == "" {
		if err = validateAuthParam(string(c.UserKey), "user_key"); err != nil {
			return err
		}
	}

	if string(c.UserKey) == "" {
		if err = validateAuthParam(c.UserKeyLocation, "user_key_file"); err != nil {
			return err
		}
	}

	return nil
}

type Discovery struct {
	*refresh.Discovery
	logger  *slog.Logger
	cfg     *SDConfig
	port    int
	metrics *chefMetrics
}

// NewDiscovery returns a new ChefDiscovery which periodically refreshes its targets.
func NewDiscovery(cfg *SDConfig, logger *slog.Logger, metrics discovery.DiscovererMetrics) (*Discovery, error) {
	m, ok := metrics.(*chefMetrics)
	if !ok {
		return nil, fmt.Errorf("invalid discovery metrics type")
	}

	if logger == nil {
		logger = promslog.NewNopLogger()
	}

	d := &Discovery{
		cfg:     cfg,
		port:    cfg.Port,
		logger:  logger,
		metrics: m,
	}

	d.Discovery = refresh.NewDiscovery(
		refresh.Options{
			Logger:              logger,
			Mech:                "chef",
			Interval:            time.Duration(cfg.RefreshInterval),
			RefreshF:            d.refresh,
			MetricsInstantiator: m.refreshMetrics,
		},
	)

	return d, nil
}

// createChefClient is a helper function for creating a Chef server connection.
func createChefClient(cfg SDConfig) (*ChefClient, error) {
	var key string

	if cfg.UserKey != "" {
		key = string(cfg.UserKey)
	} else {
		io, err := os.ReadFile(cfg.UserKeyLocation)
		if err != nil {
			return &ChefClient{}, err
		}
		key = string(io)
	}

	config := chef.Config{
		Name:    cfg.UserID,
		Key:     key,
		SkipSSL: cfg.IgnoreSSL,
		BaseURL: cfg.ChefServer,
	}

	client, err := chef.NewClient(&config)

	if err != nil {
		return &ChefClient{}, err
	}

	return &ChefClient{client}, nil
}

// virtualMachine represents a Chef node
type virtualMachine struct {
	ID        string
	URL       string
	Attribute map[string]interface{}
}

func (d *Discovery) refresh(ctx context.Context) ([]*targetgroup.Group, error) {
	defer d.logger.Debug("Chef discovery completed")

	client, err := createChefClient(*d.cfg)
	if err != nil {
		return nil, errors.Wrap(err, "could not create Chef client")
	}

	nodes, err := client.getNodes(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not get nodes")
	}

	d.logger.Debug("Found nodes during Chef discovery.", "count", len(nodes))

	tg := &targetgroup.Group{}
	for _, node := range nodes {
		if node.Attribute["ipaddress"] != nil {
			label := model.LabelSet{
				model.AddressLabel:       model.LabelValue(net.JoinHostPort(node.Attribute["ipaddress"].(string), fmt.Sprintf("%d", d.port))),
				chefLabelNodeID:          model.LabelValue(node.ID),
				chefLabelNodeURL:         model.LabelValue(node.URL),
				chefLabelNodeName:        model.LabelValue(node.Attribute["hostname"].(string)),
				chefLabelNodeOSType:      model.LabelValue(node.Attribute["os"].(string)),
				chefLabelNodeEnvironment: model.LabelValue(node.Attribute["chef_environment"].(string)),
				chefLabelNodeIP:          model.LabelValue(node.Attribute["ipaddress"].(string)),
				chefLabelNodeTag:         model.LabelValue(strings.Join(unwrapArray(node.Attribute["tags"]), ",")),
				chefLabelNodeRole:        model.LabelValue(strings.Join(unwrapArray(node.Attribute["roles"]), ",")),
			}

			for _, attr := range d.cfg.MetaAttribute {
				res := metaAttr(attr, node)
				if res != nil {
					for k, v := range attr {
						if v != nil {
							label[chefLabelNodeAttribute+model.LabelName(v.(string))] = model.LabelValue(fmt.Sprintf("%v", res))
						} else {
							label[chefLabelNodeAttribute+model.LabelName(k)] = model.LabelValue(fmt.Sprintf("%v", res))
						}
					}
				}
			}

			tg.Targets = append(tg.Targets, label)
		} else {
			d.metrics.bootstrapFailure.WithLabelValues(node.ID).Inc()
		}
	}

	return []*targetgroup.Group{tg}, nil
}

// getNodes connects to Chef Client and returns an array of virtualMachines
func (client *ChefClient) getNodes(ctx context.Context) ([]virtualMachine, error) {
	var nodes []virtualMachine

	result, err := client.Nodes.List()
	if err != nil {
		return nil, errors.Wrap(err, "could not list virtual machines")
	}

	for node, url := range result {
		v, err := client.mapFromNode(node, url)
		if err != nil {
			return nil, errors.Wrap(err, "could not list virtual machines")
		}
		nodes = append(nodes, v)
	}

	return nodes, nil
}

// mapFromNode gets passed Chef NodeID and returns captured Chef Server attributes
func (client *ChefClient) mapFromNode(node string, url string) (virtualMachine, error) {
	nodeCheck, err := client.Nodes.Get(node)
	if err != nil {
		return virtualMachine{}, errors.Wrap(err, fmt.Sprintf("could not get node attributes for %v", node))
	}

	// All Chef attribute types ordered by precedence (Last one wins)
	attributeTypes := []map[string]interface{}{nodeCheck.DefaultAttributes, nodeCheck.NormalAttributes, nodeCheck.OverrideAttributes, nodeCheck.AutomaticAttributes}
	chefAttribute := mergeMaps(attributeTypes...)

	return virtualMachine{
		ID:        node,
		URL:       url,
		Attribute: chefAttribute,
	}, nil
}

// function for unwrapping arrays into a string for label values
func unwrapArray(t interface{}) []string {
	arr := []string{}
	switch reflect.TypeOf(t).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(t)

		for i := 0; i < s.Len(); i++ {
			arr = append(arr, s.Index(i).Interface().(string))
		}
	}
	return arr
}

// function for merging multiple maps
func mergeMaps(maps ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// function for getting passed a list of chef attributes to be collected for relabelling
func metaAttr(h map[string]interface{}, n virtualMachine) interface{} {
	var res interface{}
	for k := range h {
		escape := regexp.MustCompile(`\\_`)
		escaped := escape.ReplaceAllString(k, `\\`)
		re := regexp.MustCompile(`_`)
		a := re.Split(escaped, -1)
		attr := n.Attribute

		for i, y := range a {
			a[i] = strings.ReplaceAll(y, `\\`, `_`)
		}

		for _, z := range a {
			if attr[z] != nil {
				b := reflect.TypeOf(attr[z]).Kind()
				switch b {
				case reflect.Map:
					attr = attr[z].(map[string]interface{})
				default:
					res = attr[z]
					attr = map[string]interface{}{}
				}
			}
		}
	}
	return res
}
