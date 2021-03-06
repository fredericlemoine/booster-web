# BOOSTER-WEB: Web interface to [BOOSTER](http://booster.c3bi.pasteur.fr)
![build](https://travis-ci.org/evolbioinfo/booster-web.svg?branch=master)

This interface presents informations about BOOSTER program, and allows to run BOOSTER easily.

# Installing BOOSTER-WEB
## Already compiled
Download a release in the [release](https://github.com/evolbioinfo/booster-web/releases) section. You can directly run the executable for your platform.

## From source
To compile BOOSTER-WEB, you must [download](https://golang.org/dl/) and [install](https://golang.org/doc/install) Go (version >=1.9) on your system.

Then you just have to type :
```
go get github.com/evolbioinfo/booster-web/
go get -u github.com/golang/dep/cmd/dep
```

This will download BOOSTER-WEB sources from github, and its dependencies.

You can then build BOOSTER-WEB with:
```
cd $GOPATH/src/github.com/evolbioinfo/booster-web/
dep ensure
make
```

The `booster-web` executable should be located in the `$GOPATH/bin` folder.

## From Docker

A docker image running a Galaxy server with all tools used by booster-web (PhyML-SMS, FastTree, Booster) and an already configured booster-web server is avalaible on [docker hub](https://hub.docker.com/r/evolbioinfo/booster-web/).

* Download and run already configured booster-web + galaxy servers:

```
docker run --privileged=true \
            -p 8080:80 -p 8000:8888 \
            -p 8 -p 8121:21 -p 8122:22 \
            evolbioinfo/booster-web:v0.1.8
```

Then visit [http://localhost:8000](http://localhost:8000) .


# Running BOOSTER-WEB
## Default configuration
You can directly run the `booster-web` executable without any configuration. It will setup a web server with the following default properties:
* Run on localhost, port 8080
* Log to stderr
* In memory database (analyses will not persist after server shutdown)
* Local processor (booster jobs will run on the local machine)
* 1 parallel Runner: One job at a time
* Job Timeout: unlimited
* 1 thread per job

To access the web interface, just go to [http://localhost:8080](http://localhost:8080)

Note that the local processor only allows to run booster from already inferred trees (no PhyML-SMS nor FastTree workflow).
To also run tree inference workflows, see "Other configurations", or "Install from Docker".

## Other configurations
It is possible to configure `booster-web` to run with specific options. To do so, create a configuration file `booster-web.toml` with the following sections:
* general
  * maintenance = [true|false]
* database
  * type = "[memory|mysql]"
  * user = "[mysql user]"
  * port = [mysql port]
  * host = "[mysql host]"
  * pass = "[mysql pass]"
  * dbname = "[mysql dbname]"
* itol
  * key = "[iTOL api key]"
  * project = "[itol upload project]"
* runners
  * type="[galaxy|local]"
  * queuesize=[size of job queue]
  * nbrunners=[number of parallel local runners]
  * jobthreads=[number of threads per local job]
  * timeout=[job timeout in seconds: 0=ulimited]
  * memlimit=[Max allowed Memory in Bytes]
  * keepold=[Number of days to keep results of old analyses]
* galaxy (Only used if runners.type="galaxy")
  * key="[galaxy api key]"
  * url="[url of the galaxy server: http(s)://ip:port]"
* galaxy.tools
  * booster="[Id of booster tool on the galaxy server]"
  * phyml="[Id of PHYML-SMS tool on the galaxy server]"
  * fasttree="[Id of FastTree tool on the galaxy server]"
* notification (for notification when jobs are finished)
  * activated=[true|false]
  * smtp="[smtp serveur for sending email]"
  * port=[smtp port]
  * user="[smtp user]"
  * pass="[smtp password]"
  * resultpage = "[url to result pages]"
  * sender="[sender of the notification]"
* logging
  * logfile= "[stderr|stdout|/path/to/logfile]"
* http
  * port=[http server listening port]
* authentication
  * user="[global username]"
  * password="[global password]"

And run booster web: `booster-web --config booster-web.toml`

## Example of configuration file
```
[general]
# If booster-web is in maintenance mode or not
maintenance = false

[database]
# Type : memory|mysql (default memory)
type = "mysql"
user = "mysql_user"
port = 3306
host = "mysql_server"
pass = "mysql_pass"
dbname = "mysql_db_name"

[itol]
key = "xxxxxxxxxx"
project = "booster"

[runners]
# galaxy|local if galaxy: required galaxykey & galaxyurl
type="galaxy"
# Maximum number of pending jobs (default : 10): for galaxy & local
queuesize = 200
# Number of parallel running jobs (default : 1): for local only
nbrunners  = 1
# Number of cpus per bootstrap job : for local only
jobthreads  = 10
# Timout for each job in seconds (default unlimited): for local only
#timeout  = 1000
# Memory limit in Bytes for each job (uses job memory estimation): for galaxy only
#memlimit  = 8000000000
# Keep old finished analyses for 10 days, default=0 (unlimited)
keepold = 10

#Only used if runners.type="galaxy"
[galaxy]
key="galaxy_api_key"
url="https://galaxy.server.com/"

[galaxy.tools]
# Id of booster tool on the galaxy server
booster="/.../booster/booster/version"
# Id of PhyML-SMS tool on the galaxy server
phyml="/.../phyml-sms/version"
# Id of FastTree tool on the galaxy server
fasttree="/.../fasttree/version"

# For notification when job is finished
[notification]
# true|false
activated=true
# smtp serveur for sending email
smtp="smtp.serveur.com"
# Port
port=587
# Smtp user 
user="smtp_user"
# Smtp password
pass="smtp_pass"
# booster-web server name:port/view page,
# used to give the right url in result email
resultpage = "http://url/view"
# sender of the notification
sender = "sender@server.com"

[logging]
# Log file : stdout|stderr|any file
logfile = "booster.log"

[http]
# HTTP server Listening port
port = 4000

# For running a private server, default: no authentication
#[authentication]
#user     = "user"
#password = "pass"
```

