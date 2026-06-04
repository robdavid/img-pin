package main

import (
	"github.com/robdavid/img-pin/pkgs/cmd"
	_ "github.com/robdavid/img-pin/pkgs/k8s/k3s"
	_ "github.com/robdavid/img-pin/pkgs/k8s/workload"
)

func main() {
	cmd.Main()
}
