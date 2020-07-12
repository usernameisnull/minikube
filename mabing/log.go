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
		Fmtf("错误: `%s`\n", err.Error())
	}
	if err = command.Wait(); err != nil {
		Fmtf("错误: `%s`\n", err.Error())
	} else {
		Fmtln("docker进程号: ", command.ProcessState.Pid())
		//fmt.Println(command.ProcessState.Sys().(syscall.WaitStatus).ExitCode)
		Fmtln(outinfo.String())
	}

}

func CheckPort(p ...string) {
	var port string
	if len(p) >= 1 {
		port = p[0]
	} else {
		port = "8443"
	}
	Fmtln("====================, CheckPort ", port)
	command := exec.Command("lsof", fmt.Sprintf("-i:%s", port))
	outinfo := bytes.Buffer{}
	command.Stdout = &outinfo
	err := command.Start()
	if err != nil {
		Fmtf("错误: `%s`, 可能相关的端口还没起来\n", err.Error())
	}
	if err = command.Wait(); err != nil {
		Fmtf("错误: `%s`\n", err.Error())
	} else {
		Fmtln(outinfo.String())
	}
}
