package connectortest

func init() {
	RegisterDiscoveryFixtures("proxmox",
		DiscoveryFixture{
			Name: "web ui", Port: 8006, Path: "/", Match: true,
			Response: Response(200, `<!DOCTYPE html>
<html>
  <head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
    <title>pve1 - Proxmox Virtual Environment</title>
    <link rel="icon" sizes="128x128" href="/pve2/images/logo-128.png" />
`, "Server", "pve-api-daemon/3.0", "Content-Type", "text/html; charset=UTF-8"),
		},
		DiscoveryFixture{
			Name: "server header only", Port: 8006, Path: "/", Match: true,
			Response: Response(501, "", "Server", "pve-api-daemon/3.0"),
		},
		DiscoveryFixture{
			Name: "backup server on the same port", Port: 8006, Path: "/", Match: false,
			Response: Response(200, `<html><head><title>pbs - Proxmox Backup Server</title></head></html>`),
		},
		DiscoveryFixture{
			Name: "other tls service", Port: 8006, Path: "/", Match: false,
			Response: Response(200, `<html><head><title>Cockpit</title></head></html>`, "Server", "cockpit"),
		},
	)
}
