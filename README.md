# Lastfm Stat

Microservice to fetch listening stat from [Last FM](https://last.fm)


## Run

```bash
docker run -d --name lastfm-stat tmshv/lastfm-stat
```

## API

```
GET /status
```

```
GET /user/<username>/records
```

```
GET /user/<username>/status
```

## DB

```
{
    config              // Common config
    user.<username>     // User specifiс data
        lastScan
        records
}
```


### Config

### Scan

```json5
{
  scanDate: 'Date',
  minRecordDate: 'Date',
  user: 'String',
}
```

### User
    lastScan
    records
    
### Record
