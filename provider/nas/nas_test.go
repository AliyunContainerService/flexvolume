package nas

import "testing"

func TestCheckOptions(t *testing.T) {
	plugin := &NasPlugin{}
    optin := &NasOptions{Server: "", Path: "/k8s", Vers: "4.0", Mode: "755"}
	plugin.checkOptions(optin)
}