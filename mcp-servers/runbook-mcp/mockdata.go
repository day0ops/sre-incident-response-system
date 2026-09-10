package main

type Deployment struct {
	Deployment string `json:"deployment"`
	Version    string `json:"version"`
	DeployedAt string `json:"deployedAt"`
	Status     string `json:"status"`
}

var mockDeployments = []Deployment{
	{
		Deployment: "checkout-api",
		Version:    "v42",
		DeployedAt: "8 minutes ago",
		Status:     "active",
	},
	{
		Deployment: "checkout-api",
		Version:    "v41",
		DeployedAt: "3 days ago",
		Status:     "previous",
	},
}
