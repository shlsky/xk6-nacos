package xnacos

import (
	"encoding/json"
	"errors"
	"github.com/dop251/goja"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

// NacosClient represents a gRPC client that can be used to make RPC requests
type NacosClient struct {
	nacos naming_client.INamingClient
	vu    modules.VU
	addr  string
}

func init() {

	modules.Register("k6/x/nacos", New())
}

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the GRPC module for every VU.
	ModuleInstance struct {
		vu      modules.VU
		exports map[string]interface{}
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

type NacosParams struct {
	IpAddr      string //the nacos server address
	Port        uint64 //the nacos server port
	Username    string //the username for nacos auth
	Password    string //the password for nacos auth
	Group       string //the password for nacos auth
	NamespaceId string //the namespaceId of Nacos.When namespace is public, fill in the blank string here.
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	mi := &ModuleInstance{
		vu:      vu,
		exports: make(map[string]interface{}),
	}

	mi.exports["NacosClient"] = mi.NacosClient
	return mi
}

// NacosClient is the JS constructor for the grpc NacosClient.
func (mi *ModuleInstance) NacosClient(call goja.ConstructorCall) *goja.Object {
	runtime := mi.vu.Runtime()

	var nacosParam *NacosParams
	if len(call.Arguments) == 0 {
		common.Throw(runtime, errors.New("not enough arguments"))
	}

	if params, ok := call.Argument(0).Export().(map[string]interface{}); ok {
		if b, err := json.Marshal(params); err != nil {
			common.Throw(runtime, err)
		} else {
			if err = json.Unmarshal(b, &nacosParam); err != nil {
				common.Throw(runtime, err)
			}
		}
	}
	sc := []constant.ServerConfig{
		{
			IpAddr: nacosParam.IpAddr,
			Port:   nacosParam.Port,
		},
	}

	cc := constant.ClientConfig{
		NamespaceId:         nacosParam.NamespaceId, //namespace id
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "nacos/log",
		CacheDir:            "nacos/cache",
		LogLevel:            "error",
		Username:            nacosParam.Username,
		Password:            nacosParam.Password,
	}
	// a more graceful way to create naming client
	nacos, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		},
	)
	if err != nil {
		common.Throw(runtime, err)
	}

	return runtime.ToValue(&NacosClient{vu: mi.vu, nacos: nacos}).ToObject(runtime)
}

// Exports returns the exports of the grpc module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: mi.exports,
	}
}

func (c *NacosClient) SelectOneHealthyInstance(svcName string, group string) *model.Instance {
	state := c.vu.State()
	if state == nil {
		common.Throw(c.vu.Runtime(), errors.New("connecting to a gRPC server in the init context is not supported"))
	}
	param := vo.SelectOneHealthInstanceParam{
		ServiceName: svcName,
		GroupName:   group,
	}
	res, err := c.nacos.SelectOneHealthyInstance(param)
	if err != nil {
		common.Throw(c.vu.Runtime(), err)
	}
	return res
}
