# 复用本机 Envoy 镜像中的系统 CA 与运行库；编译使用宿主已有 Go 工具链。
FROM envoyproxy/envoy:v1.39.1
COPY ingate-ai-proxy /usr/local/bin/ingate-ai-proxy
ENTRYPOINT ["/usr/local/bin/ingate-ai-proxy"]
