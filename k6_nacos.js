import nacos from "k6/x/nacos";
import {check, sleep, group} from 'k6'

var nacosClient = new nacos.NacosClient({
    ipAddr: "nacos.test.infra.ww5sawfyut0k.bitsvc.io",
    port: 8848,
    username: "nacos",
    password: "nacos",
    namespaceId: "efficiency-test",
});

export const options = {
    discardResponseBodies: false,
    scenarios: {
        scenario1: {
            executor: 'ramping-vus', stages: [{duration: '30s', target: 1}], startVUs: 1, exec: 'scenarioExec0'
        }
    }
}

export function scenarioExec0() {
    var ip = nacosClient.selectOneHealthyInstance("eff-pts-agent");
    // console.info(ip.ip,ip.port);
    // sleep(0.2);
}