package xnacos

import (
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"go.k6.io/k6/js/modules"
)

// NacosClient represents a gRPC client that can be used to make RPC requests
type NacosClient struct {
	NacosMap map[string]naming_client.INamingClient
}

func init() {

	modules.Register("k6/x/nacos", New())
}

var realNacosClient = NacosClient{
	NacosMap: make(map[string]naming_client.INamingClient),
}

func New() *NacosClient {
	return &realNacosClient
}

type NacosParams struct {
	IpAddr      string //the nacos server address
	Port        uint64 //the nacos server port
	Username    string //the username for nacos auth
	Password    string //the password for nacos auth
	Group       string //the password for nacos auth
	NamespaceId string //the namespaceId of Nacos.When namespace is public, fill in the blank string here.
}

// NacosClient is the JS constructor for the grpc NacosClient.
func (c *NacosClient) Init(nacosKey string, IpAddr string, Port uint64, Username string, Password string, NamespaceId string) error {

	sc := []constant.ServerConfig{
		{
			IpAddr: IpAddr,
			Port:   Port,
		},
	}

	cc := constant.ClientConfig{
		NamespaceId:         NamespaceId, //namespace id
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "nacos/log",
		CacheDir:            "nacos/cache",
		LogLevel:            "error",
		Username:            Username,
		Password:            Password,
	}
	// a more graceful way to create naming client
	nacos, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		},
	)

	c.NacosMap[nacosKey] = nacos

	return err
}

func (c *NacosClient) SelectOneHealthyInstance(nacosKey string, svcName string, group string) (*model.Instance, error) {

	param := vo.SelectOneHealthInstanceParam{
		ServiceName: svcName,
		GroupName:   group,
	}
	res, err := c.NacosMap[nacosKey].SelectOneHealthyInstance(param)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *NacosClient) SelectAllInstances(nacosKey string, svcName string, group string) ([]model.Instance, error) {
	param := vo.SelectAllInstancesParam{
		ServiceName: svcName,
		GroupName:   group,
	}
	res, err := c.NacosMap[nacosKey].SelectAllInstances(param)
	if err != nil {
		return nil, err
	}
	return res, nil
}
