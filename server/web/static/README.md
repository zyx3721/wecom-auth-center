# 静态文件目录

存放企业微信域名归属校验文件等需要以 `https://auth.example.com/<文件名>` 直接访问的静态资源。

## 使用方式

1. 企业微信管理后台配置可信域名时会提供 `WW_verify_xxxxxxxx.txt` 下载；
2. 将文件原样放入本目录（不要改名）；
3. 服务启动后即可通过 `https://auth.example.com/WW_verify_xxxxxxxx.txt` 访问，再到后台完成验证。

校验文件不含敏感信息，可入库；真实文件名以后台下发的为准。
