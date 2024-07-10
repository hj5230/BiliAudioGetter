# BiliAudioGetter

BiliAudioGetter 是一个用于从Bilibili下载音频的工具。

BiliAudioGetter is a tool for downloading audio from Bilibili.

## 简介 Introduction

本工具使用户能够通过URL与应用程序交互从Bilibili下载音频，并使用FFmpeg将其转换为指定的格式。

This tool enables users to download audio from Bilibili by interacting with the application via URL, and convert them to specified formats using FFmpeg.

部署在161.35.90.99/biliaudiogetter，欢迎试用。

Deployed at 161.35.90.99/biliaudiogetter, you're welcomed to try it out.

## 功能 Features

- 从Bilibili视频中提取音频
- 支持指定视频的特定分P
- 可选择音频比特率
- 支持转换为AAC或MP3格式
- 支持FLAC格式（需要登录和VIP权限）

---

- Extract audio from Bilibili videos
- Support for specific parts of a video
- Optional audio bitrate selection
- Support for conversion to AAC or MP3 formats
- Support for FLAC format (requires login and VIP privileges)

## 使用方法 Usage

### 基本用法 Basic Usage

访问以下URL来下载音频：

Visit the following URL to download audio:

```
http://[your-domain]?bv=[BV号]&p=[分P数]&bitrate=[比特率]&format=[格式]
```

参数说明：
- `bv`: Bilibili视频的BV号（必填）
- `p`: 分P数，默认为1
- `bitrate`: 音频比特率，单位为k，最大320k
- `format`: 输出格式，可选 "mp3"，默认为AAC

Parameter description:
- `bv`: Bilibili video BV number (required)
- `p`: Part number, default is 1
- `bitrate`: Audio bitrate, unit is k, maximum 320k
- `format`: Output format, "mp3" optional, default is AAC

### 示例 Examples

1. 下载默认格式（AAC）的音频：

   Download audio in default format (AAC):
   ```
   http://[your-domain]/?bv={BV}
   ```

2. 下载特定分P的MP3格式音频：

   Download MP3 format audio of a specific part:
   ```
   http://[your-domain]/?bv={BV}&p=1&format=mp3
   ```

3. 下载指定比特率的音频：

   Download audio with specified bitrate:
   ```
   http://[your-domain]/?bv=BV1xx411c7mD&bitrate=192
   ```

### API 使用 API Usage

获取视频元数据URL：

Get video metadata URL:
```
http://[your-domain]/api/?bv=[BV号]&p=[分P数]
```

### FLAC 下载 FLAC Download

需要提供source和hdnts参数（需要登录和VIP权限）：

Requires source and hdnts parameters (login and VIP privileges required):
```
http://[your-domain]/flac/?source=[source_url]&hdnts=[hdnts_value]
```

## 注意事项 Notes

- 比特率最高支持320k
- FLAC格式下载需要登录和VIP权限
- 请遵守Bilibili的使用条款和版权规定

---

- Maximum supported bitrate is 320k
- FLAC format download requires login and VIP privileges
- Please comply with Bilibili's terms of use and copyright regulations

## 依赖 Dependencies

- Go
- FFmpeg
- github.com/u2takey/ffmpeg-go

## 安装和运行 Installation and Running

1. 确保已安装Go和FFmpeg
2. 克隆仓库
3. 安装依赖：`go mod tidy`
4. 运行程序：`go run main.go`
5. 服务将在 :8081 端口运行

---

1. Ensure Go and FFmpeg are installed
2. Clone the repository
3. Install dependencies: `go mod tidy`
4. Run the program: `go run main.go`
5. The service will run on port :8081

## 许可 License

BSD 3-Clause License
