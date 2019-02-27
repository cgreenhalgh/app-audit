# Audit app design notes

Chris Greenhalgh, The University of Nottingham, 2019

Based on databox version 0.5.2-dev as of mid-Feb 2019.

## Platform support

### Store 

The store adds write provenance data to the commit messages when data is updated

what’s currently generated is supposed to be PROV format, and all interactions between components are supposed to be via Zest. so in principle it ought to be the case that all interactions are recorded in a format that allows us to apply tools to them.

This seems to be Prov.info, 
```
"event = %s, trigger = (host=%s, method=%s, format=%s, path=%s)"
```
Where event is (e.g.) “WRITE”, host is from URI, method is GET, POST, DELETE, format is json, text, binary, path is from URI.

Macaroons are checked (path, method, target,[observe]), but as bearer tokens there is no other client authentication/identification?!

There is no API to access historical audit data. (defined by Store, or implemented in libs)

Audit data for reads is not stored! It’s only assessable via the observe API, then its gone.

### Store observe API

Some docs exist [here](https://me-box.github.io/zestdb/) under Observation

an audit request can be used to provide meta-data containing information such as the hostnames involved and the type of query etc.
```
#timestamp #server-name #client-name #method #uri-path #response-code
1521553488680 Johns-MacBook-Pro.local Johns-MacBook-Pro.local POST /kv/foo/bar 65
```
Note that “client-name” is name from URI, i.e. as used by client, not a client ID. 

The only way to access the data is to observe the store with a observe-mode set to audit (this has some support in the [goZestClient](https://github.com/me-box/goZestClient/blob/master/zest.go#L209) but not in node)

this works but only in real time. You have to observe all of the stores all of the time to get the audit data. 

Not sure if this is supported in current go/node/ocaml/etc libs

### Permissions

How an app would request permissions to audit a store is not yet defined so would need define this and modify the CM

### Container Manager

Has a store (“core-store”). Registers datasources:
-	ServiceStatus (/)
-	api (/kv/api)
-	data (/kv/data)
-	slaStore (all SLAs)
and possibly (made but unused?): apps (manifests) (/kv/apps); drivers (manifests) (/kv/drivers); all (manifests) (/kv/all).

Receives commands (to restart, install, uninstall components) via store “api” datasource, which it observes.

Persists (e.g.) SLAs (under slaStore/NAME), certificates (under data/cert.pem & data/cert.der), QRCode (under data/qrcode.png), container statuses (under data/containerStatus), data sources (under data/datasources) etc. in store, making them available to core-ui.

Container manager includes HTTPS reverse proxy, which forwards requests to core-ui, apps and drivers. Doesn’t obviously log anything at the moment.

### CM API datasource

Type `databox:container-manager:api`.

defined in [cmZestAPI.go](https://github.com/me-box/core-container-manager/blob/master/cmZestAPI.go),

Store type `kv`, content type `application/json`.

Keys:
- `install`, expect JSON object w `manifest` = libDatabox.Manifest (although it is a manifest it is ready to become an SLA, e.g. datasource IDs and hrefs are present)
- `uninstall`, expect JSON object w `name` = string
- `restart`, expect JSON object w `name` = string

### CM SLA datasource

Defined in [gmStoreClient.go](https://github.com/me-box/core-container-manager/blob/master/cmStoreClient.go)

Type `databox:container-manager:SLA`

Store type `kv`, content type `json`

Key = SLA `Name`, value is [libDatabox.SLA](https://github.com/me-box/lib-go-databox/blob/master/types.go#L105)

### CM ListAllDatasources function

Type `databox:func:ListAllDatasources`, returns []libDatabox.HypercatItem.

Could be given permission from Manifest (I think).
No way to know if it has settled or changed though.
Gets Hypercat root from arbiter. I think this depends on CM's special status with arbiter.
Iterates Items and gets Store Datasource Catalogue. This CM GET /cat permission is installed for every new app/driver's store.

Not very efficient to keep polling...

Note: current fails with permission error, see [#315](https://github.com/me-box/databox/issues/315)

And what about permission to audit once you know a datasource exists?
Or permission to get or monitor a store's hypercat (e.g. for dynamic datasources)?
Is this a function call on the CM used by the audit app?
Or is it a special host wildcard permission??

FWIW the arbiter and core store probably already accept "*" for the host, to match any core store.
So it would just need a way for
- the manifest to request it
- the core-ui to approve it
- the CM to grant it

For normal datasources the core-ui uses type to identify possible datasource IDs, and fills this in in the manifest/prototype SLA.
Maybe it should be a new "special permissions" section (host, path, method, observe (optional))?
Maybe a pre-defined list of well-defined options, e.g. "Audit any datasource", "Read any datasource catalogue"?
(It doesn't really fit the current datasources section.)

## Use Cases

### Platform audit

Overhead of having audit on

Dashboard?

-	Data flow (actual vs permitted)
-	Rates of data flow
-	Changes of behaviour?
-	Possible external linkage/flow?

### Application audit

Like Platform audit but specific scope/filter.

(Driver audit)

Like App but with networking?!

## Implementation

### Audit application

Would need permission to read/observe core-store in order to monitor general Databox activity.

Would need permission to audit all other stores. Similarly would need core network access to all stores. This would need to be introduced dynamically as new stores were created.

Might need special handling in container-manager/store install handling to ensure audit is in place before app starts using store.

Would it its own store? Would it then observe itself?? 

## Background

### W3C Prov

[Overview](https://www.w3.org/TR/prov-overview/)
-	Entity wasGenerateBy Activity (opt. qualifiedGeneration with Role)
-	Activity used Entity (opt. qualifiedUsage in Role …)
-	Entity wasDerivedFrom Entity (e.g. quoted from, is revision of)
-	Entity wasAttributedTo Agent
-	Activity wasAssociatedWith Agent (opt. qualifiedAssociation in Role …)
-	Activities may follow Plans
-	Entities may have Alternates or Specialisations
-	Entities may be organised in Collections
-	Activities can be started by Entities (e.g. email triggers discussion)
-	Agents may be Software agents, Organisations or Persons

Prov tries to avoid entities changing, by linking provenance to specific Alternates or Specialisations (e.g. specific versions of) some time-varying resources.

### Possible mappings of W3C Prov to Databox

Basic app operation

A datasource could be an Entity, with specialisations corresponding to value(s) over time.

An app could be an Agent.

It may be considered to act on behalf of the user, and/or the Organisation that published it.

Generating a UI view or data export could be an activity.

A UI view could be the Entity (Specialisation) generated by that activity.

Similarly, an output datasource could also be an Entity, with specialisations corresponding to value(s) over time.

A view Entity or output datasource would therefore be Derived From the the corresponding input datasource Entities (Specialisations) via the generating activity.

For an online (event-driven) app it will be associated with a succession of Update activities, each taking (potentially) updated input Entities (datasources) and generating new version(s) of output Entities (datasources).

### App management

An app can also be viewed as an Entity. 

This allows it to be linked back to its provider.

An apps installation on a particular Databox may be a Specialisation of the app in the app store.

Installation is the Activity which Generates the local app. 

Uninstallation is the Activity which invalidates it.

