[Challenge](./challenge.md)
<br/>
<br/>

<h2  style="text-align: center;">Architecture</h2>

![Architecture Overview](./docs/architecture-overview.svg)
*Architecture Overview*
- **Assumptions**
	- For Simplicity, A single tenant application is assumed and database schema is designed accordingly.

- Using Domain Driven Design(DDD), with co-location folder structure.
	- adapters (http_handlers, repositories, dao etc) --(depends on)--> services (service) -> domain (entities, validations etc)
- A loosely coupled monolith, with separate entry points for the server and worker. Alternatively, in a large scale environment, it can be broken in to separate services - Orders Service, Products Service and Promo Service. 
- The code is organized with clear separation based on these features, making it ready to be scalable.
- #### Server:
	- A simple go server serving the apis listed in [api/openapi.yaml](api/openapi.yaml). 

- #### Worker: (Overview - "Implementation Details" in a below section)
	- Fetches the compressed coupon code files, processes and generates a single valid coupons list file. The generated file is stored as a shared volume on the host machine. Then sends a message to server, so the server can update the cache(alternatively, main server can watch processed_coupon.txt with inotify).
	- This process runs both on server startup and also while polling, when the input files are updated.
	- Will be deployed as a separate long running worker container that continuosly polls a message queue(redis in this case, for simplicity) to listen for file changes. (If files were static, this would be a single removable job).
	- Alternatively, in a production setting, the worker job can be offloaded to a serverless(eg: AWS Lambda) function, that can process and save the processed list file in AWS S3, and update the redis cluster.
- **Database:** Postgres DB is chosen for its ACID compliance (transactions for order placing and stock count reduction), strong consistency and better handling of relational data.

	![Database Schema](./docs/db-schema.png)
- **Deployment** with Docker Compose
	- Going with docker-compose for the simple use case.
	- Kuberenetes would be preferable for distributed, Multi-tenant SaaS deployments.
	- Will try to deploy to AWS ECS if time allows for it.

## Technology Choices
- Golang for backend server and worker (Required)
- Postgres - with squirrel query builder and sqlx query runner.
- Redis for Cache and as a message Queue(pub/sub)
- Gin for router.
- zap for logging
- viper for loading config
- Scalar Web client for api testing
- testify for assertions in testing
- google/uuid for generating UUIDs at application level (rather than at DB level)

### Local Deployment Steps:
- Install Docker
- In `{project root}/backend-challenge/` directory, Run `docker compose up --build`
- Port 8080 is required by the api server, can be changed in `./docker-compose.yml`
- Open the url for scalar api client, served at `http://localhost:8080/scalar` for testing the apis.
- Loading large files:
	- The couponbase files are loaded directly from the s3 (links given in [challenge.md](./challenge.md)) during runtime(simulates production architecture). Downloading large files may take time and can cause repeated failures. So, for the demo purpose, better approach would be to serve these files locally (download the files and run a static server like` python -m http.server 8080`in the directory).
	- Change the file urls env variables in docker-compose.yml to point to the local server
		```.yml
		# COUPON_FILE1_URL: https://orderfoodonline-files.s3.ap-southeast-2.amazonaws.com/couponbase1.gz
		# COUPON_FILE2_URL: https://orderfoodonline-files.s3.ap-southeast-2.amazonaws.com/couponbase2.gz
		# COUPON_FILE3_URL: https://orderfoodonline-files.s3.ap-southeast-2.amazonaws.com/couponbase3.gz
		COUPON_FILE1_URL: http://host.docker.internal:2025/couponbase1.gz #host.docker.internal = host machine's localhost
		COUPON_FILE2_URL: http://host.docker.internal:2025/couponbase2.gz
		COUPON_FILE3_URL: http://host.docker.internal:2025/couponbase3.gz
		```
	- This should let the `coupon_preprocessor` worker startup quickly.
	- The deployment has config in "development" mode. Database is cleared and seeded everytime the application is rebuilt. So, ProductIds change after re building the api service.

### System Requirements For Coupons Worker and Cache
- Naive Approach:✖️
	- Process 3 files in to 1 valid coupons file (worst case  ~200M = ~2GB), and load it to memory while processing. and store these items in to Redis.
	- Worker Job may require at most a RAM size upto 14 GB. Docker Desktop might be needed to configure to allow for higher memory allocation.
	- Worker Job Requires processing 3 * 1GB files(~100M). Based on coupon validity requirements, At most 200M coupon tokens might have to be loaded on to a golang map object. (map object = 70-80 bytes per coupon * 200 M = 14GB)
- Optimized Approach:✔️
	- Optimized approach uses a 2-tiered cache approach. A hot cache of 500K coupons (configurable) in to Redis (~50MB RAM). Remaining coupons on an indexed file, which allows for faster querying.
	- The Pre-processing is still CPU Intensive. So, Higher memory and CPU allocation is added for the container in [docker-compose.yml](docker-compose.yml). However my local system still struggles with the actual files. Works fine with smaller file sizes.
	- Requires further profiling.


### Implementation Details
- Go conventions are used for folder and package naming (smallcasenospace) and file naming (snake_case)
- Folder structure is grouped by module/service with layered approach.
- **Products Module:**
	- TDD was used to develop the apis. Integration tests were added for repositories, interacting with real database. Unit tests were added to repository and service layers.
	Pagination and other filter query params are supported by the list api. but the **response did not include pagination info**, as I did not want to change the api schema. 
- **Order Module**
	- API is authenticated, payload is validated, against formatting and stock availability. Transaction is used to create order and reduce quantity. 'FOR UPDATE' is used while reading rows for strict isolation.
- **Promo Module**
	- The module code is shared by two applications - [Server](cmd/server/main.go) and [Worker](cmd/worker/main.go). 
	- Redis Pub/Sub is used to coordinate loading of coupons in to cache. An admin api `/admin/update-coupon-cache` simulates a 'coupon files updated event' (say from s3), to which the server would send a trigger processing message to Pub/Sub(Message Queue). The Worker receives the message and creates the valid_coupons file, send a message to the Pub/Sub. The Server on receiving the message, refreshes its cache from the valid_coupons file.
	- Redis Cache is used to store hot cache of coupons
	- A processed indexed file (indexed for faster querying) is used as cold cache, which is accessed on cache-miss. The loaded coupon will now be added to hot cache. LRU eviction strategy is used. 
	- Indexed file works well and performant for this case, even compared to using a No SQL databases like cassandra or Elastic Search which are overkill.

### Note: 
#### Known Issues
Due to Time constraint, In a few areas delivering the working solution is given preference over testing, coding style consistency, and log-level correctness etc.
Few coding style issues are to be addressed. These will continued to be fixed after assignment submission.
- backend-challenge\internal\common\config\config.go has some inconsistent env loading and inconsistent env variable naming, that needs refactoring. [WILL DO]
- order_service.go has direct sql access in order to orchestrate transaction(only repositories should work with database). This is common and acceptable, but I would like to use "Unit Of Work" pattern to avoid this.
- Tests are pending for Order and promo modules
- Few logs are logged as info, instead of debug.
- sqlx struct mapping is used to read from DB, but `row.scan()` is used in few places directly.
- Building the images separately and running them separately sometimes causes database connection issues in api service. Needs to be investigated.
- These and other inconsistencies found are the most likely the result of working in isolation, time constraint and lack of automated CI/CD setup.
#### API schema Confusion
- The openapi.yml schema file present in the api/ folder of the original repository is different from the schema file linked in the [challenge](./challenge.md). I have followed the linked schema in the challenge. This schema does not return images in GET /Product api.
---
- ### [Todo](./TODO.md)

