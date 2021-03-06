# go-k8s-metadata

To build for local testing

### Docker build
- To build the docker image

```
git clone https://github.com/go-k8s-metadata.git
cd k8-proxy/go-k8s-metadata
docker build -t <docker_image_name> .
Environment variable requierd
export ADAPTATION_REQUEST_QUEUE_HOSTNAME='<rabbit-host>' \ 
ADAPTATION_REQUEST_QUEUE_PORT='<rabbit-port>' \
MESSAGE_BROKER_USER='<rabbit-user>' \
MESSAGE_BROKER_PASSWORD='<rabbit-password>' \
MINIO_ENDPOINT='<minio-endpoint>' \ 
MINIO_ACCESS_KEY='<minio-access>' \ 
MINIO_SECRET_KEY='<minio-secret>' \ 
MINIO_SOURCE_BUCKET='<bucket-to-upload-file>' \ 
MINIO_CLEAN_BUCKET='<bucket-to-upload-file>' \ 
Tika_ENDPOINT='<tika-endpoint>' 

```
### build

- First make sure that you have rabbitmq and minio running.
- For quick start using docker to run containers for RabbitMQ and MinIO.
- Run Standalone MinIO on Docker.

```
docker run -e "MINIO_ROOT_USER=<minio_root_user_name>" \
-e "MINIO_ROOT_PASSWORD=<minio_root_password>" \
-d -p 9000:9000 minio/minio server /data
```

- Run RabbitMQ on Docker.

```
docker run -d --hostname <host_name> --name <container_name> -p 15672:15672 -p 5672:5672 rabbitmq:3-management
```
```
 docker pull apache/tika:1.25
 docker run -d -p 9998:9998 apache/tika:1.25
```
# Rebuild flow to implement

![new-rebuild-flow-v2](https://github.com/k8-proxy/go-k8s-infra/raw/main/diagram/go-k8s-infra.png)
