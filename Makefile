.ONESHEL .PHONY .SILENT:

APPS=seed server

dbuser=postgres
dbpassword=postgres
dbhost=localhost
dbport=5432
dbname=cursor
conn := postgresql://$(dbuser):$(dbpassword)@$(dbhost):$(dbport)/$(dbname)?sslmode=disable

.db.create:
	docker start $(dbname) >/dev/null 2>&1 || docker run --name $(dbname) \
		-p $(dbport):5432 \
		-e POSTGRES_USER=$(dbuser) \
		-e POSTGRES_PASSWORD=$(dbpassword) \
		-e POSTGRES_DB=$(dbname) \
		-d postgres

.db.migrate:
	migrate -path ./migrations -database ${conn} up

.db.health:
	while ! pg_isready --host=${dbhost} --port=${dbport} --username=${dbuser} >/dev/null; \
	do \
		echo "waiting for database to start"; \
		sleep 3; \
	done

.db: .db.create .db.health .db.migrate

clean:
	docker kill $(dbname)
	docker rm $(dbname)

$(APPS): .db
	DB_CONNECTION=${conn} go run cmd/$@/main.go