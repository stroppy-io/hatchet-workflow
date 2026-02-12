package provision

//
//const defaultBaseImageID = "ubuntu-22-04"
//
//// PlacementConfig holds all parameters needed to convert a PlacementIntent
//// into a concrete Placement. Settings, hatchet connection, and run identity
//// are extracted from the workflow context.
//type PlacementConfig struct {
//	RunID             string
//	SelectedTarget    *settings.SelectedTarget
//	HatchetConnection *edge.HatchetConnection
//}
//
//func (c *PlacementConfig) target() deployment.Target {
//	return getDeploymentTarget(c.SelectedTarget)
//}
//
//func (c *PlacementConfig) baseImageID() string {
//	if yc := c.SelectedTarget.GetYandexCloudSettings(); yc != nil {
//		if id := yc.GetVmSettings().GetBaseImageId(); id != "" {
//			return id
//		}
//	}
//	return defaultBaseImageID
//}
//
//func (c *PlacementConfig) vmUser() *deployment.VmUser {
//	if yc := c.SelectedTarget.GetYandexCloudSettings(); yc != nil {
//		if users := yc.GetVmSettings().GetVmUsers(); len(users) > 0 {
//			return users[0]
//		}
//	}
//	return &deployment.VmUser{
//		Name:              "stroppy-edge-worker",
//		Groups:            []string{"stroppy-edge-worker"},
//		SshAuthorizedKeys: nil,
//		Sudo:              true,
//	}
//}
//
//func (c *PlacementConfig) hasPublicIp() bool {
//	if yc := c.SelectedTarget.GetYandexCloudSettings(); yc != nil {
//		return yc.GetVmSettings().GetEnablePublicIps()
//	}
//	return false
//}
//
//// BuildPlacement converts a PlacementIntent into a Placement by
//// materializing each intent item into a concrete VM template, worker,
//// and building the aggregate deployment template.
//func BuildPlacement(
//	intent *provision.PlacementIntent,
//	network *deployment.Network,
//	cfg PlacementConfig,
//) (*provision.Placement, error) {
//	if intent == nil || len(intent.GetItems()) == 0 {
//		return nil, fmt.Errorf("placement intent is empty")
//	}
//
//	items := intent.GetItems()
//	placementItems := make([]*provision.Placement_Item, 0, len(items))
//	vmTemplates := make([]*deployment.Vm_Template, 0, len(items))
//
//	for _, intentItem := range items {
//		worker := buildWorker(intentItem, cfg)
//		vmTmpl := buildVmTemplate(intentItem, worker, cfg)
//		vmTemplates = append(vmTemplates, vmTmpl)
//
//		placementItems = append(placementItems, &provision.Placement_Item{
//			Name:       intentItem.GetName(),
//			Containers: intentItem.GetContainers(),
//			VmTemplate: vmTmpl,
//			Worker:     worker,
//			Metadata:   intentItem.GetMetadata(),
//		})
//	}
//
//	deplID := ids.NewUlid().Lower().String()
//	deplTmpl := &deployment.Deployment_Template{
//		Identifier: &deployment.Identifier{
//			Id:     deplID,
//			Name:   fmt.Sprintf("deployment-%s", deplID),
//			Target: cfg.target(),
//		},
//		Network:     network,
//		VmTemplates: vmTemplates,
//	}
//
//	return &provision.Placement{
//		DeploymentTemplate: deplTmpl,
//		Items:              placementItems,
//	}, nil
//}
//
//func buildVmTemplate(
//	item *provision.PlacementIntent_Item,
//	worker *edge.Worker,
//	cfg PlacementConfig,
//) *deployment.Vm_Template {
//	vmID := ids.NewUlid().Lower().String()
//	return &deployment.Vm_Template{
//		Identifier: &deployment.Identifier{
//			Id:     vmID,
//			Name:   item.GetName(),
//			Target: cfg.target(),
//		},
//		Hardware:    item.GetHardware(),
//		BaseImageId: cfg.baseImageID(),
//		HasPublicIp: cfg.hasPublicIp(),
//		VmUser:      cfg.vmUser(),
//		InternalIp:  item.GetInternalIp(),
//		CloudInit:   buildCloudInit(worker, cfg),
//		Labels:      item.GetMetadata(),
//	}
//}
//
//func buildCloudInit(worker *edge.Worker, cfg PlacementConfig) *deployment.CloudInit {
//	env := make(map[string]string)
//
//	if conn := cfg.HatchetConnection; conn != nil {
//		switch conn.GetConnection().(type) {
//		case *edge.HatchetConnection_Url:
//			env["HATCHET_CLIENT_SERVER_URL"] = conn.GetUrl()
//		case *edge.HatchetConnection_HostPort_:
//			hp := conn.GetHostPort()
//			env["HATCHET_CLIENT_HOST_PORT"] = net.JoinHostPort(hp.GetHost(), hp.GetPort())
//		}
//		env["HATCHET_CLIENT_TOKEN"] = conn.GetToken()
//		env["HATCHET_CLIENT_TLS_STRATEGY"] = "none"
//	}
//
//	env["HATCHET_EDGE_WORKER_NAME"] = worker.GetWorkerName()
//	env["HATCHET_EDGE_ACCEPTABLE_TASKS"] = edgeDomain.TaskIdListToString(worker.GetAcceptableTasks())
//
//	return &deployment.CloudInit{
//		Env: env,
//	}
//}
//
//func buildWorker(item *provision.PlacementIntent_Item, cfg PlacementConfig) *edge.Worker {
//	runId := ids.ParseRunId(cfg.RunID)
//	kinds := inferTaskKinds(item)
//	tasks := make([]*edge.Task_Identifier, 0, len(kinds))
//	for _, kind := range kinds {
//		tasks = append(tasks, edgeDomain.NewTaskId(runId, kind))
//	}
//	return &edge.Worker{
//		WorkerName:      edgeDomain.NewWorkerName(runId, item.GetName()),
//		AcceptableTasks: tasks,
//		Metadata:        item.GetMetadata(),
//	}
//}
//
//func inferTaskKinds(item *provision.PlacementIntent_Item) []edge.Task_Kind {
//	var kinds []edge.Task_Kind
//	for _, c := range item.GetContainers() {
//		if c.GetPostgres() != nil {
//			kinds = append(kinds, edge.Task_Kind_KIND_SETUP_DATABASE_INSTANCE)
//			break
//		}
//	}
//	return kinds
//}
