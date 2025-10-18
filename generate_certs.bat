@echo off
setlocal enabledelayedexpansion

echo 🔐 Generating TLS certificates with SANs...

:: Создаем директории
if not exist "certs\ca" mkdir certs\ca
if not exist "certs\mainservice" mkdir certs\mainservice
if not exist "certs\database" mkdir certs\database
if not exist "certs\admin" mkdir certs\admin

:: Создаем конфигурационные файлы

:: CA config
echo [ req ] > certs\ca\ca.cnf
echo default_bits = 4096 >> certs\ca\ca.cnf
echo prompt = no >> certs\ca\ca.cnf
echo default_md = sha256 >> certs\ca\ca.cnf
echo distinguished_name = dn >> certs\ca\ca.cnf
echo x509_extensions = v3_ca >> certs\ca\ca.cnf
echo. >> certs\ca\ca.cnf
echo [ dn ] >> certs\ca\ca.cnf
echo C = RU >> certs\ca\ca.cnf
echo ST = Moscow >> certs\ca\ca.cnf
echo L = Moscow >> certs\ca\ca.cnf
echo O = IndustrialRegistrySystem >> certs\ca\ca.cnf
echo OU = CA >> certs\ca\ca.cnf
echo CN = Industrial Registry System CA >> certs\ca\ca.cnf
echo. >> certs\ca\ca.cnf
echo [ v3_ca ] >> certs\ca\ca.cnf
echo subjectKeyIdentifier = hash >> certs\ca\ca.cnf
echo authorityKeyIdentifier = keyid:always,issuer >> certs\ca\ca.cnf
echo basicConstraints = critical, CA:true >> certs\ca\ca.cnf
echo keyUsage = critical, digitalSignature, keyCertSign, cRLSign >> certs\ca\ca.cnf

:: MainService config
echo [ req ] > certs\mainservice\mainservice.cnf
echo default_bits = 4096 >> certs\mainservice\mainservice.cnf
echo prompt = no >> certs\mainservice\mainservice.cnf
echo default_md = sha256 >> certs\mainservice\mainservice.cnf
echo distinguished_name = dn >> certs\mainservice\mainservice.cnf
echo req_extensions = req_ext >> certs\mainservice\mainservice.cnf
echo. >> certs\mainservice\mainservice.cnf
echo [ dn ] >> certs\mainservice\mainservice.cnf
echo C = RU >> certs\mainservice\mainservice.cnf
echo ST = Moscow >> certs\mainservice\mainservice.cnf
echo L = Moscow >> certs\mainservice\mainservice.cnf
echo O = IndustrialRegistrySystem >> certs\mainservice\mainservice.cnf
echo OU = MainService >> certs\mainservice\mainservice.cnf
echo CN = mainservice >> certs\mainservice\mainservice.cnf
echo. >> certs\mainservice\mainservice.cnf
echo [ req_ext ] >> certs\mainservice\mainservice.cnf
echo subjectAltName = @alt_names >> certs\mainservice\mainservice.cnf
echo keyUsage = digitalSignature, keyEncipherment >> certs\mainservice\mainservice.cnf
echo extendedKeyUsage = serverAuth, clientAuth >> certs\mainservice\mainservice.cnf
echo. >> certs\mainservice\mainservice.cnf
echo [ alt_names ] >> certs\mainservice\mainservice.cnf
echo DNS.1 = mainservice >> certs\mainservice\mainservice.cnf
echo DNS.2 = localhost >> certs\mainservice\mainservice.cnf
echo IP.1 = 127.0.0.1 >> certs\mainservice\mainservice.cnf

:: Database config
echo [ req ] > certs\database\database.cnf
echo default_bits = 4096 >> certs\database\database.cnf
echo prompt = no >> certs\database\database.cnf
echo default_md = sha256 >> certs\database\database.cnf
echo distinguished_name = dn >> certs\database\database.cnf
echo req_extensions = req_ext >> certs\database\database.cnf
echo. >> certs\database\database.cnf
echo [ dn ] >> certs\database\database.cnf
echo C = RU >> certs\database\database.cnf
echo ST = Moscow >> certs\database\database.cnf
echo L = Moscow >> certs\database\database.cnf
echo O = IndustrialRegistrySystem >> certs\database\database.cnf
echo OU = Database >> certs\database\database.cnf
echo CN = database >> certs\database\database.cnf
echo. >> certs\database\database.cnf
echo [ req_ext ] >> certs\database\database.cnf
echo subjectAltName = @alt_names >> certs\database\database.cnf
echo keyUsage = digitalSignature, keyEncipherment >> certs\database\database.cnf
echo extendedKeyUsage = serverAuth, clientAuth >> certs\database\database.cnf
echo. >> certs\database\database.cnf
echo [ alt_names ] >> certs\database\database.cnf
echo DNS.1 = database >> certs\database\database.cnf
echo DNS.2 = localhost >> certs\database\database.cnf
echo IP.1 = 127.0.0.1 >> certs\database\database.cnf

:: Admin config
echo [ req ] > certs\admin\admin.cnf
echo default_bits = 4096 >> certs\admin\admin.cnf
echo prompt = no >> certs\admin\admin.cnf
echo default_md = sha256 >> certs\admin\admin.cnf
echo distinguished_name = dn >> certs\admin\admin.cnf
echo req_extensions = req_ext >> certs\admin\admin.cnf
echo. >> certs\admin\admin.cnf
echo [ dn ] >> certs\admin\admin.cnf
echo C = RU >> certs\admin\admin.cnf
echo ST = Moscow >> certs\admin\admin.cnf
echo L = Moscow >> certs\admin\admin.cnf
echo O = IndustrialRegistrySystem >> certs\admin\admin.cnf
echo OU = Admin >> certs\admin\admin.cnf
echo CN = admin >> certs\admin\admin.cnf
echo. >> certs\admin\admin.cnf
echo [ req_ext ] >> certs\admin\admin.cnf
echo subjectAltName = @alt_names >> certs\admin\admin.cnf
echo keyUsage = digitalSignature, keyEncipherment >> certs\admin\admin.cnf
echo extendedKeyUsage = serverAuth, clientAuth >> certs\admin\admin.cnf
echo. >> certs\admin\admin.cnf
echo [ alt_names ] >> certs\admin\admin.cnf
echo DNS.1 = admin >> certs\admin\admin.cnf
echo DNS.2 = localhost >> certs\admin\admin.cnf
echo IP.1 = 127.0.0.1 >> certs\admin\admin.cnf

:: Генерируем CA ключ и сертификат
echo 📝 Generating CA certificate...
openssl genrsa -out certs\ca\ca.key 4096
if %errorlevel% neq 0 (
    echo ❌ Failed to generate CA key
    exit /b 1
)

openssl req -new -x509 -days 3650 -key certs\ca\ca.key -out certs\ca\ca.crt -config certs\ca\ca.cnf
if %errorlevel% neq 0 (
    echo ❌ Failed to generate CA certificate
    exit /b 1
)

:: Генерируем сертификат для mainservice
echo 📝 Generating mainservice certificate...
openssl genrsa -out certs\mainservice\mainservice.key 4096
if %errorlevel% neq 0 (
    echo ❌ Failed to generate mainservice key
    exit /b 1
)

openssl req -new -key certs\mainservice\mainservice.key -out certs\mainservice\mainservice.csr -config certs\mainservice\mainservice.cnf
if %errorlevel% neq 0 (
    echo ❌ Failed to generate mainservice CSR
    exit /b 1
)

openssl x509 -req -in certs\mainservice\mainservice.csr -CA certs\ca\ca.crt -CAkey certs\ca\ca.key -CAcreateserial -out certs\mainservice\mainservice.crt -days 365 -extfile certs\mainservice\mainservice.cnf -extensions req_ext
if %errorlevel% neq 0 (
    echo ❌ Failed to generate mainservice certificate
    exit /b 1
)

:: Создаем fullchain для mainservice
type certs\mainservice\mainservice.crt certs\ca\ca.crt > certs\mainservice\mainservice-fullchain.crt

:: Генерируем сертификат для database
echo 📝 Generating database certificate...
openssl genrsa -out certs\database\database.key 4096
if %errorlevel% neq 0 (
    echo ❌ Failed to generate database key
    exit /b 1
)

openssl req -new -key certs\database\database.key -out certs\database\database.csr -config certs\database\database.cnf
if %errorlevel% neq 0 (
    echo ❌ Failed to generate database CSR
    exit /b 1
)

openssl x509 -req -in certs\database\database.csr -CA certs\ca\ca.crt -CAkey certs\ca\ca.key -CAcreateserial -out certs\database\database.crt -days 365 -extfile certs\database\database.cnf -extensions req_ext
if %errorlevel% neq 0 (
    echo ❌ Failed to generate database certificate
    exit /b 1
)

:: Создаем fullchain для database
type certs\database\database.crt certs\ca\ca.crt > certs\database\database-fullchain.crt

:: Генерируем сертификат для admin
echo 📝 Generating admin certificate...
openssl genrsa -out certs\admin\admin.key 4096
if %errorlevel% neq 0 (
    echo ❌ Failed to generate admin key
    exit /b 1
)

openssl req -new -key certs\admin\admin.key -out certs\admin\admin.csr -config certs\admin\admin.cnf
if %errorlevel% neq 0 (
    echo ❌ Failed to generate admin CSR
    exit /b 1
)

openssl x509 -req -in certs\admin\admin.csr -CA certs\ca\ca.crt -CAkey certs\ca\ca.key -CAcreateserial -out certs\admin\admin.crt -days 365 -extfile certs\admin\admin.cnf -extensions req_ext
if %errorlevel% neq 0 (
    echo ❌ Failed to generate admin certificate
    exit /b 1
)

:: Создаем fullchain для admin
type certs\admin\admin.crt certs\ca\ca.crt > certs\admin\admin-fullchain.crt

echo ✅ Certificates generated successfully!
echo    CA: certs\ca\ca.crt
echo    MainService: certs\mainservice\mainservice-fullchain.crt
echo    Database: certs\database\database-fullchain.crt
echo    Admin: certs\admin\admin-fullchain.crt

echo.
echo 🔒 Certificate details:
echo CA Certificate:
openssl x509 -in certs\ca\ca.crt -subject -noout
echo.
echo MainService Certificate:
openssl x509 -in certs\mainservice\mainservice.crt -subject -noout
openssl x509 -in certs\mainservice\mainservice.crt -text -noout | findstr "Subject Alternative Name"
echo.
echo Database Certificate:
openssl x509 -in certs\database\database.crt -subject -noout
openssl x509 -in certs\database\database.crt -text -noout | findstr "Subject Alternative Name"
echo.
echo Admin Certificate:
openssl x509 -in certs\admin\admin.crt -subject -noout
openssl x509 -in certs\admin\admin.crt -text -noout | findstr "Subject Alternative Name"

endlocal