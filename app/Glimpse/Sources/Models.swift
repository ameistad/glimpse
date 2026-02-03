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
    }

    var fileSizeFormatted: String {
        let formatter = ByteCountFormatter()
        formatter.countStyle = .file
        return formatter.string(fromByteCount: fileSize)
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
    let totalFolders: Int
    let totalOriginalMB: Int64

    enum CodingKeys: String, CodingKey {
        case totalPhotos = "total_photos"
        case totalFolders = "total_folders"
        case totalOriginalMB = "total_original_mb"
    }
}
