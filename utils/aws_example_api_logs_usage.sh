aws --profile lfproduct-dev dynamodb put-item \
  --table-name cla-dev-api-log \
  --region us-east-1 \
  --item '{
    "url": {"S": "/health"},
    "dt": {"N": "1715086845123"},
    "bucket": {"S": "ALL"}
  }'

aws --profile lfproduct-dev dynamodb query \
  --table-name cla-dev-api-log \
  --region us-east-1 \
  --key-condition-expression "#u = :u" \
  --expression-attribute-names '{"#u":"url"}' \
  --expression-attribute-values '{":u":{"S":"/health"}}'
{
    "Items": [
        {
            "url": {
                "S": "/health"
            },
            "bucket": {
                "S": "ALL"
            },
            "dt": {
                "N": "1715086845123"
            }
        }
    ],
    "Count": 1,
    "ScannedCount": 1,
    "ConsumedCapacity": null
}

aws --profile lfproduct-dev dynamodb query \
  --table-name cla-dev-api-log \
  --index-name bucket-dt-index \
  --region us-east-1 \
  --key-condition-expression "#b = :b AND #dt BETWEEN :f AND :t" \
  --expression-attribute-names '{"#b":"bucket","#dt":"dt"}' \
  --expression-attribute-values '{
    ":b":{"S":"ALL"},
    ":f":{"N":"0"},
    ":t":{"N":"9999999999999"}
  }'
{
    "Items": [
        {
            "dt": {
                "N": "1715086845123"
            },
            "url": {
                "S": "/health"
            },
            "bucket": {
                "S": "ALL"
            }
        }
    ],
    "Count": 1,
    "ScannedCount": 1,
    "ConsumedCapacity": null
}
