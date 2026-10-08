package connectortest

func init() {
	RegisterDiscoveryFixtures("pbs",
		DiscoveryFixture{
			Name: "web ui", Port: 8007, Path: "/", Match: true,
			Response: Response(200, `<!DOCTYPE html>
<html>
  <head>
    <title>pbs1 - Proxmox Backup Server</title>
  </head>
`, "Content-Type", "text/html; charset=UTF-8"),
		},
		DiscoveryFixture{
			Name: "proxmox ve on the same port", Port: 8007, Path: "/", Match: false,
			Response: Response(200, `<html><head><title>pve1 - Proxmox Virtual Environment</title></head></html>`, "Server", "pve-api-daemon/3.0"),
		},
		DiscoveryFixture{
			Name: "other web ui", Port: 8007, Path: "/", Match: false,
			Response: Response(200, `<html><head><title>Cockpit</title></head></html>`),
		},
	)
}
