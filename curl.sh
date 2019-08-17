curl -v \
'https://api.wallapop.com/api/v3/general/search?density_type=20&filters_source=search_box&keywords=7700k+-6600&language=es_ES&latitude=40.4893538&longitude=-3.6827461&num_results=15&search_id=c40084a2-f326-4aaf-848f-a98409cf83c3&start=0&step=7' \
--compressed \
-H 'User-Agent: Mozilla/5.0 (X11; Linux x86_64; rv:69.0) Gecko/20100101 Firefox/69.0' \
-H 'Accept: application/json, text/plain, */*' \
-H 'Timestamp: 1566080226615' \
-H 'X-Signature: qM0FBwHTwYlrXN9AwgnY8vSutLe11PpChgxvKlDQdgg=' \
-H 'Connection: close' \
| jq . | tee items.json
