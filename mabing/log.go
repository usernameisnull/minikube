package mabing

import (
	"fmt"

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
