import SwiftUI

struct PhotoDetailView: View {
    let photo: Photo
    let thumbnailURL: URL?
    let onDownload: () -> Void

    @State private var isDownloading = false

    var body: some View {
        VStack(spacing: 0) {
            // Large preview
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

            // Info panel
            VStack(alignment: .leading, spacing: 12) {
                // Filename
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
                        // Reset after a delay (actual completion is async)
                        DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
                            isDownloading = false
                        }
                    }) {
                        if isDownloading {
                            ProgressView()
                                .controlSize(.small)
                        } else {
                            Label("Download RAW", systemImage: "arrow.down.circle")
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .disabled(isDownloading)
                }

                Divider()

                // Metadata grid
                LazyVGrid(columns: [
                    GridItem(.flexible()),
                    GridItem(.flexible())
                ], alignment: .leading, spacing: 8) {
                    MetadataRow(label: "Size", value: photo.fileSizeFormatted)
                    MetadataRow(label: "Format", value: photo.extension.uppercased())

                    if let width = photo.width, let height = photo.height, width > 0, height > 0 {
                        MetadataRow(label: "Dimensions", value: "\(width) × \(height)")
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
