import Foundation

struct Photo: Codable, Identifiable, Hashable {
    let id: Int
    let originalPath: String
    let thumbnailPath: String
    let folder: String
    let filename: String
    let `extension`: String
    let fileSize: Int64
    let modTime: Date
    let width: Int?
    let height: Int?
    let createdAt: Date
    let mediaType: String
    let duration: Double?
    let videoCodec: String?
    let audioCodec: String?
    let framerate: Double?

    enum CodingKeys: String, CodingKey {
        case id
        case originalPath = "original_path"
        case thumbnailPath = "thumbnail_path"
        case folder
        case filename
        case `extension`
        case fileSize = "file_size"
        case modTime = "mod_time"
        case width
        case height
        case createdAt = "created_at"
        case mediaType = "media_type"
        case duration
        case videoCodec = "video_codec"
        case audioCodec = "audio_codec"
        case framerate
    }

    var isVideo: Bool { mediaType == "video" }

    var isNativelyPlayable: Bool {
        guard isVideo else { return false }
        let playable = [".mp4", ".mov", ".m4v"]
        return playable.contains(self.extension.lowercased())
    }

    var fileSizeFormatted: String {
        let formatter = ByteCountFormatter()
        formatter.countStyle = .file
        return formatter.string(fromByteCount: fileSize)
    }

    var durationFormatted: String? {
        guard let duration = duration, duration > 0 else { return nil }
        let total = Int(duration)
        let minutes = total / 60
        let seconds = total % 60
        if minutes > 0 {
            return String(format: "%d:%02d", minutes, seconds)
        }
        return String(format: "0:%02d", seconds)
    }
}

struct Folder: Codable, Identifiable, Hashable {
    let path: String
    let photoCount: Int

    var id: String { path }

    enum CodingKeys: String, CodingKey {
        case path
        case photoCount = "photo_count"
    }

    var displayName: String {
        if path.isEmpty {
            return "Root"
        }
        return (path as NSString).lastPathComponent
    }
}

struct Stats: Codable {
    let totalPhotos: Int
    let totalVideos: Int
    let totalFolders: Int
    let totalOriginalMB: Int64

    enum CodingKeys: String, CodingKey {
        case totalPhotos = "total_photos"
        case totalVideos = "total_videos"
        case totalFolders = "total_folders"
        case totalOriginalMB = "total_original_mb"
    }
}
