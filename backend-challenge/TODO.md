## Project Setup
- ~~Create basic server~~
- ~~Setup docker for the server~~
- ~~Setup a logger~~
- Add CI/CD [WILL DO]
	- static analysis with semgrep rules (for security and code quality)
	- tests
	- test deployment to ECS
- ~~Add database with seed-data, and connnect to the server~~
- Security: [Not implementing]
	- audit docker images
	- audit go dependancies
- ~~custom errors~~

## Implement APIs with TDD, covering all the edge cases.

### GET List Products api✔️
- No auth
- No middleware - can introduce pagination as middleware
- Response should include metadata like total, nextUrl etc; default pagination to 25
- Response Headers
- validate pagination values - 0 < pageSize < 100; page >= 0
- Happy path: return list with pagination
- ~~Add Filter query params for name and categoryId (migrate to create categories table)~~
- ~~Add sort params by name~~
- Test for empty array if db is empty. (status=200)
- Test for 500 errors
- Ratelimit
### GET Product api✔️
- No auth
- No middleware
- No validation
- Happy path: return product details
- Test for non-existent id (status=404)
- Test for 500 errors
- Ratelimit
### POST Create Order api✔️
- ~~Authenticate with middleware~~
- ~~Validate schema~~
- Route Handler:
	- ~~validate that productIds are valid~~
	- ~~validate quantity is in stock~~
	- ~~validate couponcode~~
		- ~~check from redis cache~~
		- ~~on Cache-Miss, read from valid coupons list file.~~
- Ratelimit
### PUT Update cache api✔️
- authenticate with middleware using admin api key
- read from processed-coupons and update cache
- 10 bytes(10 chars) * 100 = 1000 bytes = 1KB
- 10 bytes(10 chars) * 100000 = 1000 KB = 1MB
- 10 bytes(10 chars) * 100M = 1000 MB = 1GB
- store 100M in cache (profile/Monitor to adjust the number)
- on cache-miss, read from rest of the file.
- Tests

### POST new-coupons api✔️
- validate with admin api key
- add a message to redis message queue
- Tests

### Coupon codes Pre processing
- A worker with separate entry point, running on separate container
- poll the message queue for input file changes
- Load files from s3 links given in [challenge.md](./challenge.md) (added as env variables in docker-compose)
- read files to generate a `map[string]int`, where 8 <= len(key) <= 10 && map[key] >= 2
- store the keys in to a new `processed_coupons.txt` on to shared volume
- call an api to main server to refresh redis cache (alternatively, main server can listen to change event on processed_coupon.txt with inotify)
- **Challenges**
	- Each file is around 1GB in size. Job must not go out of memory.
	- Should not effect the application Startup time.