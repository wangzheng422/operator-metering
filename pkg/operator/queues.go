package operator

import (
	"reflect"
	"time"

	_ "github.com/prestodb/presto-go-client/presto"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	cbTypes "github.com/operator-framework/operator-metering/pkg/apis/metering/v1alpha1"
	_ "github.com/operator-framework/operator-metering/pkg/util/reflector/prometheus" // for prometheus metric registration
	_ "github.com/operator-framework/operator-metering/pkg/util/workqueue/prometheus" // for prometheus metric registration
)

func (op *Reporting) shutdownQueues() {
	for _, queue := range op.queueList {
		queue.ShutDown()
	}
}

func (op *Reporting) addReport(obj interface{}) {
	report := obj.(*cbTypes.Report)
	if report.DeletionTimestamp != nil {
		op.deleteReport(report)
		return
	}
	op.logger.Infof("adding Report %s/%s", report.Namespace, report.Name)
	op.enqueueReport(report)
}

func (op *Reporting) updateReport(prev, cur interface{}) {
	prevReport := prev.(*cbTypes.Report)
	curReport := cur.(*cbTypes.Report)
	if curReport.DeletionTimestamp != nil {
		op.deleteReport(curReport)
		return
	}

	if curReport.ResourceVersion == prevReport.ResourceVersion {
		// Periodic resyncs will send update events for all known Reports.
		// Two different versions of the same report will always have
		// different ResourceVersions.
		op.logger.Debugf("Report %s/%s resourceVersion is unchanged, skipping update", curReport.Namespace, curReport.Name)
		return
	}

	if reflect.DeepEqual(prevReport.Spec, curReport.Spec) {
		op.logger.Debugf("Report %s/%s spec is unchanged, skipping update", curReport.Namespace, curReport.Name)
		return
	}

	op.logger.Infof("updating Report %s/%s", curReport.Namespace, curReport.Name)
	op.enqueueReport(curReport)
}

func (op *Reporting) deleteReport(obj interface{}) {
	report, ok := obj.(*cbTypes.Report)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			op.logger.WithFields(log.Fields{"report": report.Name, "namespace": report.Namespace}).Errorf("Couldn't get object from tombstone %#v", obj)
			return
		}
		report, ok = tombstone.Obj.(*cbTypes.Report)
		if !ok {
			op.logger.WithFields(log.Fields{"report": report.Name, "namespace": report.Namespace}).Errorf("Tombstone contained object that is not a Report %#v", obj)
			return
		}
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(report)
	if err != nil {
		op.logger.WithFields(log.Fields{"report": report.Name, "namespace": report.Namespace}).WithError(err).Errorf("couldn't get key for object: %#v", report)
		return
	}
	op.reportQueue.Add(key)
}

func (op *Reporting) enqueueReport(report *cbTypes.Report) {
	key, err := cache.MetaNamespaceKeyFunc(report)
	if err != nil {
		op.logger.WithError(err).Errorf("Couldn't get key for object %#v: %v", report, err)
		return
	}
	op.reportQueue.Add(key)
}

func (op *Reporting) enqueueReportAfter(report *cbTypes.Report, duration time.Duration) {
	key, err := cache.MetaNamespaceKeyFunc(report)
	if err != nil {
		op.logger.WithError(err).Errorf("Couldn't get key for object %#v: %v", report, err)
		return
	}
	op.reportQueue.AddAfter(key, duration)
}

func (op *Reporting) addReportDataSource(obj interface{}) {
	ds := obj.(*cbTypes.ReportDataSource)
	if ds.DeletionTimestamp != nil {
		op.deleteReportDataSource(ds)
		return
	}
	op.logger.Infof("adding ReportDataSource %s/%s", ds.Namespace, ds.Name)
	op.enqueueReportDataSource(ds)
}

func (op *Reporting) updateReportDataSource(prev, cur interface{}) {
	curReportDataSource := cur.(*cbTypes.ReportDataSource)
	prevReportDataSource := prev.(*cbTypes.ReportDataSource)
	if curReportDataSource.DeletionTimestamp != nil {
		op.deleteReportDataSource(curReportDataSource)
		return
	}

	// we allow periodic resyncs to trigger ReportDataSources even
	// if they're not changed to ensure failed ones eventually get re-tried.
	// however, if we know that it's a Prometheus ReportDataSource where the
	// MetricImportStatus is all that changed, we can safely assume that this
	// update came from the operator updating that field and we can ignore this
	// update.
	isProm := curReportDataSource.Spec.PrometheusMetricsImporter != nil
	if isProm {
		sameSpec := reflect.DeepEqual(curReportDataSource.Spec, prevReportDataSource.Spec)
		importStatusChanged := !reflect.DeepEqual(curReportDataSource.Status.PrometheusMetricsImportStatus, prevReportDataSource.Status.PrometheusMetricsImportStatus)
		if sameSpec && importStatusChanged {
			return
		}
	}

	op.logger.Infof("updating ReportDataSource %s/%s", curReportDataSource.Namespace, curReportDataSource.Name)
	op.enqueueReportDataSource(curReportDataSource)
}

func (op *Reporting) deleteReportDataSource(obj interface{}) {
	dataSource, ok := obj.(*cbTypes.ReportDataSource)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			op.logger.WithFields(log.Fields{"reportDataSsource": dataSource.Name, "namespace": dataSource.Namespace}).Errorf("Couldn't get object from tombstone %#v", obj)
			return
		}
		dataSource, ok = tombstone.Obj.(*cbTypes.ReportDataSource)
		if !ok {
			op.logger.WithFields(log.Fields{"reportDataSsource": dataSource.Name, "namespace": dataSource.Namespace}).Errorf("Tombstone contained object that is not a ReportDataSource %#v", obj)
			return
		}
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(dataSource)
	if err != nil {
		op.logger.WithFields(log.Fields{"reportDataSsource": dataSource.Name, "namespace": dataSource.Namespace}).WithError(err).Errorf("couldn't get key for object: %#v", dataSource)
		return
	}
	op.reportDataSourceQueue.Add(key)
}

func (op *Reporting) enqueueReportDataSource(ds *cbTypes.ReportDataSource) {
	key, err := cache.MetaNamespaceKeyFunc(ds)
	if err != nil {
		op.logger.WithFields(log.Fields{"reportDataSource": ds.Name, "namespace": ds.Namespace}).WithError(err).Errorf("couldn't get key for object: %#v", ds)
		return
	}
	op.reportDataSourceQueue.Add(key)
}

func (op *Reporting) enqueueReportDataSourceAfter(ds *cbTypes.ReportDataSource, duration time.Duration) {
	key, err := cache.MetaNamespaceKeyFunc(ds)
	if err != nil {
		op.logger.WithFields(log.Fields{"reportDataSource": ds.Name, "namespace": ds.Namespace}).WithError(err).Errorf("couldn't get key for object: %#v", ds)
		return
	}
	op.reportDataSourceQueue.AddAfter(key, duration)
}

func (op *Reporting) addReportQuery(obj interface{}) {
	query := obj.(*cbTypes.ReportQuery)
	op.logger.Infof("adding ReportQuery %s/%s", query.Namespace, query.Name)
	op.enqueueReportQuery(query)
}

func (op *Reporting) updateReportQuery(prev, cur interface{}) {
	curReportQuery := cur.(*cbTypes.ReportQuery)
	prevReportQuery := prev.(*cbTypes.ReportQuery)
	logger := op.logger.WithFields(log.Fields{"reportQuery": curReportQuery.Name, "namespace": curReportQuery.Namespace})

	// Only skip queuing if we're not missing a view
	if curReportQuery.ResourceVersion == prevReportQuery.ResourceVersion {
		// Periodic resyncs will send update events for all known ReportQuerys.
		// Two different versions of the same reportQuery will always have
		// different ResourceVersions.
		logger.Debugf("ReportQuery %s/%s resourceVersion is unchanged, skipping update", curReportQuery.Namespace, curReportQuery.Name)
		return
	}
	if reflect.DeepEqual(prevReportQuery.Spec, curReportQuery.Spec) {
		logger.Debugf("ReportQuery %s/%s spec is unchanged, skipping update", curReportQuery.Namespace, curReportQuery.Name)
		return
	}

	logger.Infof("updating ReportQuery %s/%s", curReportQuery.Namespace, curReportQuery.Name)
	op.enqueueReportQuery(curReportQuery)
}

func (op *Reporting) enqueueReportQuery(query *cbTypes.ReportQuery) {
	key, err := cache.MetaNamespaceKeyFunc(query)
	if err != nil {
		op.logger.WithFields(log.Fields{"reportQuery": query.Name, "namespace": query.Namespace}).WithError(err).Errorf("couldn't get key for object: %#v", query)
		return
	}
	op.reportQueryQueue.Add(key)
}

func (op *Reporting) addPrestoTable(obj interface{}) {
	table := obj.(*cbTypes.PrestoTable)
	if table.DeletionTimestamp != nil {
		op.deletePrestoTable(table)
		return
	}
	logger := op.logger.WithFields(log.Fields{"prestoTable": table.Name, "namespace": table.Namespace})
	logger.Infof("adding PrestoTable %s/%s", table.Namespace, table.Name)
	op.enqueuePrestoTable(table)
}

func (op *Reporting) updatePrestoTable(_, cur interface{}) {
	curPrestoTable := cur.(*cbTypes.PrestoTable)
	if curPrestoTable.DeletionTimestamp != nil {
		op.deletePrestoTable(curPrestoTable)
		return
	}
	logger := op.logger.WithFields(log.Fields{"prestoTable": curPrestoTable.Name, "namespace": curPrestoTable.Namespace})
	logger.Infof("updating PrestoTable %s/%s", curPrestoTable.Namespace, curPrestoTable.Name)
	op.enqueuePrestoTable(curPrestoTable)
}

func (op *Reporting) deletePrestoTable(obj interface{}) {
	prestoTable, ok := obj.(*cbTypes.PrestoTable)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			op.logger.WithFields(log.Fields{"prestoTable": prestoTable.Name, "namespace": prestoTable.Namespace}).Errorf("Couldn't get object from tombstone %#v", obj)
			return
		}
		prestoTable, ok = tombstone.Obj.(*cbTypes.PrestoTable)
		if !ok {
			op.logger.WithFields(log.Fields{"prestoTable": prestoTable.Name, "namespace": prestoTable.Namespace}).Errorf("Tombstone contained object that is not a PrestoTable %#v", obj)
			return
		}
	}
	// when finalizers aren't enabled, it's pretty likely by the time our
	// worker get the event from the queue that the resource will no longer
	// exist in our store, so we eagerly drop the table upon seeing the delete
	// event when finalizers are disabled
	if !op.cfg.EnableFinalizers && prestoTable != nil {
		// TODO: fix
		_ = op.dropPrestoTable(prestoTable)
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(prestoTable)
	if err != nil {
		op.logger.WithFields(log.Fields{"prestoTable": prestoTable.Name, "namespace": prestoTable.Namespace}).WithError(err).Errorf("couldn't get key for object: %#v", prestoTable)
		return
	}
	op.prestoTableQueue.Add(key)
}

func (op *Reporting) enqueuePrestoTable(table *cbTypes.PrestoTable) {
	key, err := cache.MetaNamespaceKeyFunc(table)
	if err != nil {
		op.logger.WithFields(log.Fields{"prestoTable": table.Name, "namespace": table.Namespace}).WithError(err).Errorf("couldn't get key for object: %#v", table)
		return
	}
	op.prestoTableQueue.Add(key)
}

func (op *Reporting) addHiveTable(obj interface{}) {
	table := obj.(*cbTypes.HiveTable)
	if table.DeletionTimestamp != nil {
		op.deleteHiveTable(table)
		return
	}
	logger := op.logger.WithFields(log.Fields{"hiveTable": table.Name, "namespace": table.Namespace})
	logger.Infof("adding HiveTable %s/%s", table.Namespace, table.Name)
	op.enqueueHiveTable(table)
}

func (op *Reporting) updateHiveTable(_, cur interface{}) {
	curHiveTable := cur.(*cbTypes.HiveTable)
	if curHiveTable.DeletionTimestamp != nil {
		op.deleteHiveTable(curHiveTable)
		return
	}
	logger := op.logger.WithFields(log.Fields{"hiveTable": curHiveTable.Name, "namespace": curHiveTable.Namespace})
	logger.Infof("updating HiveTable %s/%s", curHiveTable.Namespace, curHiveTable.Name)
	op.enqueueHiveTable(curHiveTable)
}

func (op *Reporting) deleteHiveTable(obj interface{}) {
	hiveTable, ok := obj.(*cbTypes.HiveTable)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			op.logger.WithFields(log.Fields{"hiveTable": hiveTable.Name, "namespace": hiveTable.Namespace}).Errorf("Couldn't get object from tombstone %#v", obj)
			return
		}
		hiveTable, ok = tombstone.Obj.(*cbTypes.HiveTable)
		if !ok {
			op.logger.WithFields(log.Fields{"hiveTable": hiveTable.Name, "namespace": hiveTable.Namespace}).Errorf("Tombstone contained object that is not a HiveTable %#v", obj)
			return
		}
	}
	// when finalizers aren't enabled, it's pretty likely by the time our
	// worker get the event from the queue that the resource will no longer
	// exist in our store, so we eagerly drop the table upon seeing the delete
	// event when finalizers are disabled
	if !op.cfg.EnableFinalizers && hiveTable != nil {
		_ = op.dropHiveTable(hiveTable)
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(hiveTable)
	if err != nil {
		op.logger.WithFields(log.Fields{"hiveTable": hiveTable.Name, "namespace": hiveTable.Namespace}).WithError(err).Errorf("couldn't get key for object: %#v", hiveTable)
		return
	}
	op.hiveTableQueue.Add(key)
}

func (op *Reporting) enqueueHiveTable(table *cbTypes.HiveTable) {
	key, err := cache.MetaNamespaceKeyFunc(table)
	if err != nil {
		op.logger.WithFields(log.Fields{"hiveTable": table.Name, "namespace": table.Namespace}).WithError(err).Errorf("couldn't get key for object: %#v", table)
		return
	}
	op.hiveTableQueue.Add(key)
}

func (op *Reporting) addStorageLocation(obj interface{}) {
	storageLocation := obj.(*cbTypes.StorageLocation)
	if storageLocation.DeletionTimestamp != nil {
		op.deleteStorageLocation(storageLocation)
		return
	}
	logger := op.logger.WithFields(log.Fields{"storageLocation": storageLocation.Name, "namespace": storageLocation.Namespace})
	logger.Infof("adding StorageLocation %s/%s", storageLocation.Namespace, storageLocation.Name)
	op.enqueueStorageLocation(storageLocation)
}

func (op *Reporting) updateStorageLocation(_, cur interface{}) {
	curStorageLocation := cur.(*cbTypes.StorageLocation)
	if curStorageLocation.DeletionTimestamp != nil {
		op.deleteStorageLocation(curStorageLocation)
		return
	}
	logger := op.logger.WithFields(log.Fields{"storageLocation": curStorageLocation.Name, "namespace": curStorageLocation.Namespace})
	logger.Infof("updating StorageLocation %s/%s", curStorageLocation.Namespace, curStorageLocation.Name)
	op.enqueueStorageLocation(curStorageLocation)
}

func (op *Reporting) deleteStorageLocation(obj interface{}) {
	storageLocation, ok := obj.(*cbTypes.StorageLocation)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			op.logger.WithFields(log.Fields{"storageLocation": storageLocation.Name, "namespace": storageLocation.Namespace}).Errorf("Couldn't get object from tombstone %#v", obj)
			return
		}
		storageLocation, ok = tombstone.Obj.(*cbTypes.StorageLocation)
		if !ok {
			op.logger.WithFields(log.Fields{"storageLocation": storageLocation.Name, "namespace": storageLocation.Namespace}).Errorf("Tombstone contained object that is not a StorageLocation %#v", obj)
			return
		}
	}
	// when finalizers aren't enabled, it's pretty likely by the time our
	// worker get the event from the queue that the resource will no longer
	// exist in our store, so we eagerly drop the storageLocation upon seeing the delete
	// event when finalizers are disabled
	if !op.cfg.EnableFinalizers && storageLocation != nil {
		_ = op.deleteStorage(storageLocation)
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(storageLocation)
	if err != nil {
		op.logger.WithFields(log.Fields{"storageLocation": storageLocation.Name, "namespace": storageLocation.Namespace}).WithError(err).Errorf("couldn't get key for object: %#v", storageLocation)
		return
	}
	op.storageLocationQueue.Add(key)
}

func (op *Reporting) enqueueStorageLocation(storageLocation *cbTypes.StorageLocation) {
	key, err := cache.MetaNamespaceKeyFunc(storageLocation)
	if err != nil {
		op.logger.WithFields(log.Fields{"storageLocation": storageLocation.Name, "namespace": storageLocation.Namespace}).WithError(err).Errorf("couldn't get key for object: %#v", storageLocation)
		return
	}
	op.storageLocationQueue.Add(key)
}

type workerProcessFunc func(logger log.FieldLogger) bool

func (op *Reporting) processResource(logger log.FieldLogger, handlerFunc syncHandler, objType string, queue workqueue.RateLimitingInterface, maxRequeues int) bool {
	obj, quit := queue.Get()
	if quit {
		logger.Infof("queue is shutting down, exiting %s worker", objType)
		return false
	}
	defer queue.Done(obj)

	op.runHandler(logger, handlerFunc, objType, obj, queue, maxRequeues)
	return true
}

type syncHandler func(logger log.FieldLogger, key string) error

func (op *Reporting) runHandler(logger log.FieldLogger, handlerFunc syncHandler, objType string, obj interface{}, queue workqueue.RateLimitingInterface, maxRequeues int) {
	logger = logger.WithFields(newLogIdentifier(op.rand))
	if key, ok := op.getKeyFromQueueObj(logger, objType, obj, queue); ok {
		logger.Infof("syncing %s %s", objType, key)
		err := handlerFunc(logger, key)
		op.handleErr(logger, err, objType, key, queue, maxRequeues)
	}
}

// getKeyFromQueueObj tries to convert the object from the queue into a string,
// and if it isn't, it forgets the key from the queue, and logs an error.
//
// We expect strings to come off the workqueue. These are of the
// form namespace/name. We do this as the delayed nature of the
// workqueue means the items in the informer cache may actually be
// more up to date that when the item was initially put onto the
// workqueue.
func (op *Reporting) getKeyFromQueueObj(logger log.FieldLogger, objType string, obj interface{}, queue workqueue.RateLimitingInterface) (string, bool) {
	if key, ok := obj.(string); ok {
		return key, ok
	}
	queue.Forget(obj)
	logger.WithField(objType, obj).Errorf("expected string in work queue but got %#v", obj)
	return "", false
}

// handleErr checks if an error happened and makes sure we will retry later.
func (op *Reporting) handleErr(logger log.FieldLogger, err error, objType string, obj interface{}, queue workqueue.RateLimitingInterface, maxRequeues int) {
	logger = logger.WithField(objType, obj)

	if err == nil {
		logger.Infof("successfully synced %s %q", objType, obj)
		queue.Forget(obj)
		return
	}

	// This controller retries up to maxRequeues times if something goes wrong.
	// After that, it stops trying.
	if queue.NumRequeues(obj) < maxRequeues {
		logger.WithError(err).Errorf("error syncing %s %q, adding back to queue", objType, obj)
		queue.AddRateLimited(obj)
		return
	}

	queue.Forget(obj)
	logger.WithError(err).Infof("error syncing %s %q, dropping out of the queue", objType, obj)
}

// inTargetNamespaceEventHandlerFunc wraps a cache.ResourceEventHandler and
// only runs the wrapped handler if the resource is listed in targetNamespaces.
type inTargetNamespaceResourceEventHandler struct {
	handler          cache.ResourceEventHandler
	targetNamespaces []string
}

func (handler *inTargetNamespaceResourceEventHandler) inTargetNamespace(obj interface{}) bool {
	metav1Obj, ok := obj.(metav1.Object)
	if !ok {
		return false
	}

	for _, targetNS := range handler.targetNamespaces {
		if metav1Obj.GetNamespace() == targetNS {
			return true
		}
	}
	return false
}

func (handler *inTargetNamespaceResourceEventHandler) OnAdd(obj interface{}) {
	if handler.inTargetNamespace(obj) {
		handler.handler.OnAdd(obj)
	}
}

func (handler *inTargetNamespaceResourceEventHandler) OnUpdate(oldObj, newObj interface{}) {
	if handler.inTargetNamespace(oldObj) && handler.inTargetNamespace(newObj) {
		handler.handler.OnUpdate(oldObj, newObj)
	}
}

func (handler *inTargetNamespaceResourceEventHandler) OnDelete(obj interface{}) {
	if handler.inTargetNamespace(obj) {
		handler.handler.OnDelete(obj)
	}
}

// newInTargetNamespaceEventHandler returns an
// inTargetNamespaceResourceEventHandler to wrap the given handler. If
// targetNamespaces is empty, then it returns the original eventHandler
// unmodified.
func newInTargetNamespaceEventHandler(handler cache.ResourceEventHandler, targetNamespaces []string) cache.ResourceEventHandler {
	if len(targetNamespaces) == 0 {
		return handler
	}
	return &inTargetNamespaceResourceEventHandler{handler: handler, targetNamespaces: targetNamespaces}
}
