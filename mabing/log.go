package mabing

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/golang/glog"
)

var Logln = glog.Infoln
var Logf = glog.Infof

var longSign = "===================="

func GenerateLongSignStart(s string) string {
	return fmt.Sprintf("%s mabing, %s start %s ", longSign, s, longSign)
}

func GenerateLongSignEnd(s string) string {
	return fmt.Sprintf("%s mabing, %s end %s ", longSign, s, longSign)
}

func Fmtln(a ...interface{}) {
	fmt.Println("mabing, ", a)
}

func Fmtf(format string, a ...interface{}) {
	fmt.Printf("mabing, "+format, a)
}

func CheckDocker() {
	Fmtln("====================, CheckDocker ")
	// docker ps --format "{{.Names}}"
	command := exec.Command("docker", "ps", "--format", `"{{.Names}}"`)
	outinfo := bytes.Buffer{}
	command.Stdout = &outinfo
	err := command.Start()
	if err != nil {
		Fmtln(err.Error())
	}
	if err = command.Wait(); err != nil {
		Fmtln(err.Error())
	} else {
		Fmtln("进程号: ", command.ProcessState.Pid())
		//fmt.Println(command.ProcessState.Sys().(syscall.WaitStatus).ExitCode)
		Fmtln(outinfo.String())
	}

}
