[Challenge](./challenge.md)

## Architecture:
![Architecture Overview](./docs/architecture-overview.svg)
*Architecture Overview*
- Assuming the project is of a single-tenant(restaurant), for simplicity.
- Using Domain Driven Design(DDD), with co-location folder structure.
	- adapters (http_handlers, db_repositories, db_models etc) --(depends on)--> services (service, dto) -> domain (entities, repositories, validations etc)
- A loosely coupled monolith, with separate entry points for the server and workers.
- #### Worker: 
	- Loads the compressed coupon code files, processes and generates a single valid coupons list file. The generated file is stored as a shared volume on the host machine. Then triggers a http call to server, so the server can update the cache(alternatively, main server can watch processed_coupon.txt with inotify).
	- This process runs both on server startup and also on-demand, when the input files are updated.
	- Will be deployed as a separate long running worker container that continuosly polls a message queue(redis in this case, for simplicity) to listen for file changes. (If files were static, this would be a single removable job).
	- Alternatively, in a production setting, the worker job can be offloaded to an serverless(eg: AWS Lambda) function, that can process and save the processed list file in AWS S3, and update the redis cluster.
- **Database:** Postgres DB is chosen for its ACID compliance (transactions for order placing and stock count reduction), strong consistency and better handling of relational data.
- **Deployment** with Docker Compose
	- Going with docker-compose for the simple use case.
	- Kuberenetes would be preferable for distributed, Multi-tenant SaaS deployments.
- A UI for scalar api client, served at `http://localhost:8080/scalar` - for easy evaluation of the task.


## Technology Choices
- Golang for backend server and worker (Required)
- Postgres - with squirrel sql query builder. 
- Redis for Cache and as a message Queue
- Gin for router.
- zap for logging
- viper for loading config





## [Todo](./TODO.md)
- [In Progress](./IN_PROGRESS.md)
- [Done](./DONE.md)

