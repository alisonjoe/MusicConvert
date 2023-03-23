# MusicConvert

## 需要安装的工具
- flac
- cuetools




shnsplit -f  1.cue -t "%n.%t" -o flac BTHVN\ 2020\ -\ CD\ 001.flac -d outPath

输出文件名可利用 -t 进行格式化 (%p 艺术家, %a 专辑, %t 标题, 以及 %n 轨数):




### 拷贝metadata
ffmpeg -i a.wav -map_metadata 0  aaaa.flac

### 查看信息
ffmpeg -i a.wav -map_metadata 0 -hide_banner
ffprobe -show_format /Volumes/Music/Test/a.wav

### 提取 metadata 到文件
ffmpeg -i INPUT.FLAC -f ffmetadata FFMETADATAFILE
从 FFMETADATAFILE 文件重新插入编辑过的元数据信息可以通过以下方式完成：
ffmpeg -i INPUT -i FFMETADATAFILE -map_metadata 1 -codec copy OUTPUT

- 以 json 格式输出
ffprobe -v quiet -print_format json -show_format input.flac