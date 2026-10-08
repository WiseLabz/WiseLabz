package connectortest

func init() {
	RegisterDiscoveryFixtures("unifi",
		DiscoveryFixture{
			Name: "status", Port: 8443, Path: "/status", Match: true,
			Response: Response(200, `{"meta":{"rc":"ok","up":true,"server_version":"8.0.28","uuid":"2f8a1c3e-9d4b-4e6a-8f10-3b7c5d9e1a22"},"data":[]}`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			// Status 200 is used here so that the rc check alone rejects it.
			Name: "login required", Port: 8443, Path: "/status", Match: false,
			Response: Response(200, `{"meta":{"rc":"error","msg":"api.err.LoginRequired"},"data":[]}`, "Content-Type", "application/json"),
		},
		DiscoveryFixture{
			Name: "ok without version or uuid", Port: 8443, Path: "/status", Match: false,
			Response: Response(200, `{"meta":{"rc":"ok"},"data":[]}`, "Content-Type", "application/json"),
		},
	)
}
