/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"k8s.io/minikube/mabing"

	"github.com/docker/machine/libmachine"
	"github.com/docker/machine/libmachine/state"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/minikube/pkg/minikube/bootstrapper/bsutil/kverify"
	"k8s.io/minikube/pkg/minikube/cluster"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/exit"
	"k8s.io/minikube/pkg/minikube/kubeconfig"
	"k8s.io/minikube/pkg/minikube/machine"
	"k8s.io/minikube/pkg/minikube/mustload"
	"k8s.io/minikube/pkg/minikube/node"
)

var statusFormat string
var output string

const (
	// # Additional states used by kubeconfig:

	// Configured means configured
	Configured = "Configured" // ~state.Saved
	// Misconfigured means misconfigured
	Misconfigured = "Misconfigured" // ~state.Error

	// # Additional states used for clarity:

	// Nonexistent means nonexistent
	Nonexistent = "Nonexistent" // ~state.None
	// Irrelevant is used for statuses that aren't meaningful for worker nodes
	Irrelevant = "Irrelevant"
)

// Status holds string representations of component states
type Status struct {
	Name       string
	Host       string
	Kubelet    string
	APIServer  string
	Kubeconfig string
	Worker     bool
}

const (
	minikubeNotRunningStatusFlag = 1 << 0
	clusterNotRunningStatusFlag  = 1 << 1
	k8sNotRunningStatusFlag      = 1 << 2
	defaultStatusFormat          = `{{.Name}}
type: Control Plane
host: {{.Host}}
kubelet: {{.Kubelet}}
apiserver: {{.APIServer}}
kubeconfig: {{.Kubeconfig}}

`
	workerStatusFormat = `{{.Name}}
type: Worker
host: {{.Host}}
kubelet: {{.Kubelet}}

`
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Gets the status of a local Kubernetes cluster",
	Long: `Gets the status of a local Kubernetes cluster.
	Exit status contains the status of minikube's VM, cluster and Kubernetes encoded on it's bits in this order from right to left.
	Eg: 7 meaning: 1 (for minikube NOK) + 2 (for cluster NOK) + 4 (for Kubernetes NOK)`,
	Run: func(cmd *cobra.Command, args []string) {
		mabing.Logln(mabing.GenerateLongSignStart("status"))
		if output != "text" && statusFormat != defaultStatusFormat {
			exit.UsageT("Cannot use both --output and --format options")
		}

		cname := ClusterFlagValue()
		api, cc := mustload.Partial(cname)

		var statuses []*Status
		mabing.Logf(`mabing, Run(), 第一个条件 = %+v`, nodeName != "" || statusFormat != defaultStatusFormat && len(cc.Nodes) > 1)
		if nodeName != "" || statusFormat != defaultStatusFormat && len(cc.Nodes) > 1 {
			n, _, err := node.Retrieve(*cc, nodeName)
			if err != nil {
				exit.WithError("retrieving node", err)
			}

			st, err := status(api, *cc, *n)
			if err != nil {
				glog.Errorf("status error: %v", err)
			}
			statuses = append(statuses, st)
		} else {
			for _, n := range cc.Nodes {
				glog.Infof("checking status of %s ...", n.Name)
				machineName := driver.MachineName(*cc, n)
				st, err := status(api, *cc, n)
				glog.Infof("%s status: %+v", machineName, st)

				if err != nil {
					glog.Errorf("status error: %v", err)
				}
				if st.Host == Nonexistent {
					glog.Errorf("The %q host does not exist!", machineName)
				}
				statuses = append(statuses, st)
			}
		}

		switch strings.ToLower(output) {
		case "text":
			for _, st := range statuses {
				if err := statusText(st, os.Stdout); err != nil {
					exit.WithError("status text failure", err)
				}
			}
		case "json":
			if err := statusJSON(statuses, os.Stdout); err != nil {
				exit.WithError("status json failure", err)
			}
		default:
			exit.WithCodeT(exit.BadUsage, fmt.Sprintf("invalid output format: %s. Valid values: 'text', 'json'", output))
		}
		mabing.Logln(mabing.GenerateLongSignEnd("status"))
		os.Exit(exitCode(statuses))
	},
}

func exitCode(statuses []*Status) int {
	c := 0
	for _, st := range statuses {
		if st.Host != state.Running.String() {
			c |= minikubeNotRunningStatusFlag
		}
		if (st.APIServer != state.Running.String() && st.APIServer != Irrelevant) || st.Kubelet != state.Running.String() {
			c |= clusterNotRunningStatusFlag
		}
		if st.Kubeconfig != Configured && st.Kubeconfig != Irrelevant {
			c |= k8sNotRunningStatusFlag
		}
	}
	return c
}

func status(api libmachine.API, cc config.ClusterConfig, n config.Node) (*Status, error) {

	controlPlane := n.ControlPlane
	name := driver.MachineName(cc, n)

	st := &Status{
		Name:       name,
		Host:       Nonexistent,
		APIServer:  Nonexistent,
		Kubelet:    Nonexistent,
		Kubeconfig: Nonexistent,
		Worker:     !controlPlane,
	}

	hs, err := machine.Status(api, name)
	glog.Infof("%s host status = %q (err=%v)", name, hs, err)
	if err != nil {
		return st, errors.Wrap(err, "host")
	}

	// We have no record of this host. Return nonexistent struct
	if hs == state.None.String() {
		return st, nil
	}
	st.Host = hs

	// If it's not running, quickly bail out rather than delivering conflicting messages
	if st.Host != state.Running.String() {
		glog.Infof("host is not running, skipping remaining checks")
		st.APIServer = st.Host
		st.Kubelet = st.Host
		st.Kubeconfig = st.Host
		return st, nil
	}

	// We have a fully operational host, now we can check for details
	if _, err := cluster.DriverIP(api, name); err != nil {
		glog.Errorf("failed to get driver ip: %v", err)
		st.Host = state.Error.String()
		return st, err
	}

	st.Kubeconfig = Configured
	if !controlPlane {
		st.Kubeconfig = Irrelevant
		st.APIServer = Irrelevant
	}

	host, err := machine.LoadHost(api, name)
	if err != nil {
		return st, err
	}

	cr, err := machine.CommandRunner(host)
	if err != nil {
		return st, err
	}

	stk := kverify.KubeletStatus(cr)
	glog.Infof("%s kubelet status = %s", name, stk)
	st.Kubelet = stk.String()

	// Early exit for worker nodes
	if !controlPlane {
		return st, nil
	}

	hostname, _, port, err := driver.ControlPlaneEndpoint(&cc, &n, host.DriverName)
	if err != nil {
		glog.Errorf("forwarded endpoint: %v", err)
		st.Kubeconfig = Misconfigured
	} else {
		err := kubeconfig.VerifyEndpoint(cc.Name, hostname, port)
		if err != nil {
			glog.Errorf("kubeconfig endpoint: %v", err)
			st.Kubeconfig = Misconfigured
		}
	}

	sta, err := kverify.APIServerStatus(cr, hostname, port)
	glog.Infof("%s apiserver status = %s (err=%v)", name, stk, err)

	if err != nil {
		glog.Errorln("Error apiserver status:", err)
		st.APIServer = state.Error.String()
	} else {
		st.APIServer = sta.String()
	}

	return st, nil
}

func init() {
	statusCmd.Flags().StringVarP(&statusFormat, "format", "f", defaultStatusFormat,
		`Go template format string for the status output.  The format for Go templates can be found here: https://golang.org/pkg/text/template/
For the list accessible variables for the template, see the struct values here: https://godoc.org/k8s.io/minikube/cmd/minikube/cmd#Status`)
	statusCmd.Flags().StringVarP(&output, "output", "o", "text",
		`minikube status --output OUTPUT. json, text`)
	statusCmd.Flags().StringVarP(&nodeName, "node", "n", "", "The node to check status for. Defaults to control plane. Leave blank with default format for status on all nodes.")
}

func statusText(st *Status, w io.Writer) error {
	tmpl, err := template.New("status").Parse(statusFormat)
	if st.Worker && statusFormat == defaultStatusFormat {
		tmpl, err = template.New("worker-status").Parse(workerStatusFormat)
	}
	if err != nil {
		return err
	}
	if err := tmpl.Execute(w, st); err != nil {
		return err
	}
	if st.Kubeconfig == Misconfigured {
		_, err := w.Write([]byte("\nWARNING: Your kubectl is pointing to stale minikube-vm.\nTo fix the kubectl context, run `minikube update-context`\n"))
		return err
	}
	return nil
}

func statusJSON(st []*Status, w io.Writer) error {
	var js []byte
	var err error
	// Keep backwards compat with single node clusters to not break anyone
	if len(st) == 1 {
		js, err = json.Marshal(st[0])
	} else {
		js, err = json.Marshal(st)
	}
	if err != nil {
		return err
	}
	_, err = w.Write(js)
	return err
}
