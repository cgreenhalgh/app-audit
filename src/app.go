package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"time"
	"encoding/json"
	"strconv"
	
	"net/http"

	"github.com/gorilla/mux"

	libDatabox "github.com/me-box/lib-go-databox"
)

// from cmZestAPI.go
//data required for an install request
type installRequest struct {
	Manifest libDatabox.Manifest `json:"manifest"`
}

type restartRequest struct {
	Name string `json:"name"`
}

type uninstallRequest struct {
	Name string `json:"name"`
}


func main() {
	libDatabox.Info("Starting ....")

	//Are we running inside databox?
	DataboxTestMode := os.Getenv("DATABOX_VERSION") == ""

	//Read in the information on the datasources that databox passed to the app
	httpServerPort := "8080"
	if DataboxTestMode {
		log.Fatal("Missing DATASOURCE_VERSION assuming we are outside of databox - this won't work with this app!")
	}
	//turn on debug output for the databox library
	libDatabox.OutputDebug(true)

	//This is the standard setup for inside databox
	// Container Manager API
	var err error
	var cmapiDataSource libDatabox.DataSourceMetadata
	var cmapiStoreEndpoint string
	var cmapiStoreClient *libDatabox.CoreStoreClient
	cmapiDataSource, cmapiStoreEndpoint, err = libDatabox.HypercatToDataSourceMetadata(os.Getenv("DATASOURCE_cmapi"))
	libDatabox.ChkErr(err)
	// Set up a store client you will need one of these per store
	// if you asked for more then one data source in your manifest
	// there will be more then one env var provided by databox DATASOURCE_[manifest client id]
	cmapiStoreClient = libDatabox.NewDefaultCoreStoreClient(cmapiStoreEndpoint)

	// Container Manager SLA store
	var cmslasDataSource libDatabox.DataSourceMetadata
	var cmslasStoreEndpoint string
	var cmslasStoreClient *libDatabox.CoreStoreClient
	cmslasDataSource, cmslasStoreEndpoint, err = libDatabox.HypercatToDataSourceMetadata(os.Getenv("DATASOURCE_cmslas"))
	libDatabox.ChkErr(err)
	cmslasStoreClient = libDatabox.NewDefaultCoreStoreClient(cmslasStoreEndpoint)

	// Container Manager ListAllDatasources function
	var cmlistdssDataSource libDatabox.DataSourceMetadata
	var cmlistdssStoreEndpoint string
	var cmlistdssStoreClient *libDatabox.CoreStoreClient
	libDatabox.Info("Listdss: " + os.Getenv("DATASOURCE_listdss") )
	cmlistdssDataSource, cmlistdssStoreEndpoint, err = libDatabox.HypercatToDataSourceMetadata(os.Getenv("DATASOURCE_listdss"))
	libDatabox.ChkErr(err)
	cmlistdssStoreClient = libDatabox.NewDefaultCoreStoreClient(cmlistdssStoreEndpoint)

	//The endpoints and routing for the app UI
	router := mux.NewRouter()
	router.HandleFunc("/status", statusEndpoint).Methods("GET")
	//router.HandleFunc("/ui/getData", getData(cmapiDataSource, cmapiStoreClient)).Methods("GET")
	router.HandleFunc("/ui/crash", crashApp).Methods("GET")
	router.HandleFunc("/ui/qstest", qstest).Methods("GET")
	router.PathPrefix("/ui").Handler(http.StripPrefix("/ui", http.FileServer(http.Dir("./static"))))

	go getSLAs(cmslasStoreClient, cmslasDataSource.DataSourceID)
	go monitorCmapi(cmapiStoreClient, cmapiDataSource.DataSourceID)
	go listAllDatasources(cmlistdssStoreClient, cmlistdssDataSource.DataSourceID)
	
	//setup webserver
	setUpWebServer(DataboxTestMode, router, httpServerPort)

	libDatabox.Info("Exiting ....")
}

func qstest(w http.ResponseWriter, r *http.Request) {
	libDatabox.Info(r.URL.Path)
	libDatabox.Info(r.URL.RawPath)
	libDatabox.Info(r.URL.RawQuery)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("active\n"))
}

func crashApp(w http.ResponseWriter, r *http.Request) {
	os.Exit(2)
}

func statusEndpoint(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("active\n"))
}

func getData(dataSource libDatabox.DataSourceMetadata, store *libDatabox.CoreStoreClient) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		// API store is KV but only really observable, so sort this out in the future...
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"status":500,"data":"%s"}`, "Unimplemented")

	}
}

func setUpWebServer(testMode bool, r *mux.Router, port string) {

	srv := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  30 * time.Second,
		Handler:      r,
	}

	if testMode {
		//set up an http server for testing
		libDatabox.Info("Waiting for http requests on port http://127.0.0.1" + srv.Addr + "/ui ....")
		log.Fatal(srv.ListenAndServe())
	} else {
		//Start up a well behaved HTTPS server for displying the UI
		tlsConfig := &tls.Config{
			PreferServerCipherSuites: true,
			CurvePreferences: []tls.CurveID{
				tls.CurveP256,
			},
		}

		srv.TLSConfig = tlsConfig

		libDatabox.Info("Waiting for https requests on port " + srv.Addr + " ....")
		log.Fatal(srv.ListenAndServeTLS(libDatabox.GetHttpsCredentials(), libDatabox.GetHttpsCredentials()))
	}
}

func getSLAs(cmslasStoreClient *libDatabox.CoreStoreClient, cmslasID string) {
	var slaList []libDatabox.SLA

	keys, err := cmslasStoreClient.KVJSON.ListKeys(cmslasID)
	if err != nil {
		libDatabox.Err("[monitorActivity] Failed to list keys Error getting keys from SLA Store")
	} else {

		for _, k := range keys {
			var sla libDatabox.SLA
			payload, err := cmslasStoreClient.KVJSON.Read(cmslasID, k)
			if err != nil {
				libDatabox.Err("[monitorActivity] failed to get SLA " + k + ". " + err.Error())
				continue
			}
			err = json.Unmarshal(payload, &sla)
			if err != nil {
				libDatabox.Err("[monitorActivity] failed decode SLA for " + k + ". " + err.Error())
				continue
			}
			libDatabox.Info("Found SLA for " + k + " with " + strconv.FormatInt( int64( len(sla.Datasources) ), 10 ) + " datasources")
			
			slaList = append(slaList, sla)
		}
	}
}



func monitorCmapi(cmapiStoreClient *libDatabox.CoreStoreClient, cmapiID string) {
	ObserveResponseChan, err := cmapiStoreClient.KVJSON.Observe(cmapiID)
	libDatabox.ChkErr(err)
	if err != nil {
		libDatabox.Err("failed to observer Container Manager API. " + err.Error())
	} else {
		for {
			select {
			case ObserveResponse := <-ObserveResponseChan:
				if ObserveResponse.Key == "install" {
					var installData installRequest
					err := json.Unmarshal(ObserveResponse.Data, &installData)
					if err == nil && installData.Manifest.Name != "" {
						libDatabox.Info("install " + installData.Manifest.Name )
					} else if err == nil {
						libDatabox.Err("Install command saw invalid JSON manifest/manifest.name is blank")
					} else {
						libDatabox.Err("Install command saw invalid JSON " + err.Error())
					}
				}
				if ObserveResponse.Key == "restart" {
					var request restartRequest
					err := json.Unmarshal(ObserveResponse.Data, &request)
					libDatabox.ChkErr(err)
					if err == nil && request.Name != "" {
						libDatabox.Info("restart " + request.Name )
					} else if err == nil {
						libDatabox.Err("Restart command saw invalid JSON request.name is blank")
					} else {
						libDatabox.Err("Restart command saw invalid JSON " + err.Error())
					}
				}
				if ObserveResponse.Key == "uninstall" {
					var request uninstallRequest
					err := json.Unmarshal(ObserveResponse.Data, &request)
					libDatabox.ChkErr(err)
					if err == nil && request.Name != "" {
						libDatabox.Info("uninstall " + request.Name )
					} else if err == nil {
						libDatabox.Err("Uninstall command saw invalid JSON request.name is blank")
					} else {
						libDatabox.Err("Uninstall command saw invalid JSON " + err.Error())
					}
				}
			}
		}
	}
}

func listAllDatasources(cmlistdssStoreClient *libDatabox.CoreStoreClient, cmlistdssID string) {
	respChan, err := cmlistdssStoreClient.FUNC.Call(cmlistdssID, []byte{}, libDatabox.ContentTypeJSON)
	if err != nil {
		libDatabox.Err("listAllDatasources error. " + err.Error() )
		return
	}

	resp := <-respChan

	if resp.Status != libDatabox.FuncStatusOK {
		libDatabox.Err("listAllDatasources error status." )
		return
	}

	libDatabox.Info("listAllDatasources: " + string( resp.Response[:] ) )
}