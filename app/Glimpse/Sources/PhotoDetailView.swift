import SwiftUI
import AVKit

struct PhotoDetailView: View {
    let photo: Photo
    let thumbnailURL: URL?
    let streamURL: URL?
    let onDownload: () -> Void

    @State private var isDownloading = false
    @State private var player: AVPlayer?

    var body: some View {
        VStack(spacing: 0) {
            if photo.isVideo && photo.isNativelyPlayable, let streamURL = streamURL {
                VideoPlayer(player: player)
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .onAppear {
                        player = AVPlayer(url: streamURL)
                    }
                    .onDisappear {
                        player?.pause()
                        player = nil
                    }
                    .onChange(of: photo.id) { _, _ in
                        player?.pause()
                        if let streamURL = self.streamURL {
                            player = AVPlayer(url: streamURL)
                        }
                    }
            } else if photo.isVideo {
                ZStack {
                    AsyncImage(url: thumbnailURL) { phase in
                        switch phase {
                        case .empty:
                            ProgressView()
                                .frame(maxWidth: .infinity, maxHeight: .infinity)
                        case .success(let image):
                            image
                                .resizable()
                                .aspectRatio(contentMode: .fit)
                                .frame(maxWidth: .infinity, maxHeight: .infinity)
                        case .failure:
                            VStack {
                                Image(systemName: "film")
                                    .font(.system(size: 64))
                                    .foregroundColor(.secondary)
                                Text("Failed to load preview")
                                    .foregroundColor(.secondary)
                            }
                            .frame(maxWidth: .infinity, maxHeight: .infinity)
                        @unknown default:
                            EmptyView()
                        }
                    }

                    VStack(spacing: 8) {
                        Image(systemName: "play.slash")
                            .font(.system(size: 48))
                            .foregroundColor(.white)
                        Text("Download to play this format")
                            .font(.callout)
                            .foregroundColor(.white)
                    }
                    .padding()
                    .background(.black.opacity(0.5))
                    .cornerRadius(12)
                }
                .background(Color.black.opacity(0.05))
            } else {
                AsyncImage(url: thumbnailURL) { phase in
                    switch phase {
                    case .empty:
                        ProgressView()
                            .frame(maxWidth: .infinity, maxHeight: .infinity)
                    case .success(let image):
                        image
                            .resizable()
                            .aspectRatio(contentMode: .fit)
                            .frame(maxWidth: .infinity, maxHeight: .infinity)
                    case .failure:
                        VStack {
                            Image(systemName: "photo")
                                .font(.system(size: 64))
                                .foregroundColor(.secondary)
                            Text("Failed to load preview")
                                .foregroundColor(.secondary)
                        }
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                    @unknown default:
                        EmptyView()
                    }
                }
                .background(Color.black.opacity(0.05))
            }

            VStack(alignment: .leading, spacing: 12) {
                HStack {
                    VStack(alignment: .leading, spacing: 2) {
                        Text(photo.filename)
                            .font(.headline)
                            .lineLimit(2)

                        if !photo.folder.isEmpty {
                            Text(photo.folder)
                                .font(.caption)
                                .foregroundColor(.secondary)
                        }
                    }

                    Spacer()

                    Button(action: {
                        isDownloading = true
                        onDownload()
                        DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
                            isDownloading = false
                        }
                    }) {
                        if isDownloading {
                            ProgressView()
                                .controlSize(.small)
                        } else {
                            Label(photo.isVideo ? "Download Video" : "Download RAW",
                                  systemImage: "arrow.down.circle")
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(isDownloading)
                }

                Divider()

                LazyVGrid(columns: [
                    GridItem(.flexible()),
                    GridItem(.flexible())
                ], alignment: .leading, spacing: 8) {
                    MetadataRow(label: "Size", value: photo.fileSizeFormatted)
                    MetadataRow(label: "Format", value: photo.extension.uppercased())

                    if let width = photo.width, let height = photo.height, width > 0, height > 0 {
                        MetadataRow(label: "Dimensions", value: "\(width) \u{00d7} \(height)")
                    }

                    if photo.isVideo {
                        if let duration = photo.durationFormatted {
                            MetadataRow(label: "Duration", value: duration)
                        }
                        if let codec = photo.videoCodec, !codec.isEmpty {
                            MetadataRow(label: "Video Codec", value: codec.uppercased())
                        }
                        if let codec = photo.audioCodec, !codec.isEmpty {
                            MetadataRow(label: "Audio Codec", value: codec.uppercased())
                        }
                        if let fps = photo.framerate, fps > 0 {
                            MetadataRow(label: "Framerate", value: String(format: "%.1f fps", fps))
                        }
                    }

                    MetadataRow(label: "Modified", value: formatDate(photo.modTime))
                }
            }
            .padding()
            .background(Color(NSColor.controlBackgroundColor))
        }
    }

    private func formatDate(_ date: Date) -> String {
        let formatter = DateFormatter()
        formatter.dateStyle = .medium
        formatter.timeStyle = .short
        return formatter.string(from: date)
    }
}

struct MetadataRow: View {
    let label: String
    let value: String

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(label)
                .font(.caption)
                .foregroundColor(.secondary)
            Text(value)
                .font(.callout)
        }
    }
}
