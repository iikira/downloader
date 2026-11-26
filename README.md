# downloader
多线程下载器

# Usage
```
downloader -p=<parallel> <downloadUrl1> <downloadUrl2>...
```

下载前会通过 GET 请求获取远端文件大小, 并与本地已存在文件对比打印诊断信息 (仅诊断, 不会跳过下载); 检测到断点续传信息文件 (`<文件名>.downloader_downloading`) 时也会提示。
