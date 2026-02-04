import SwiftUI

struct SidebarView: View {
    let folders: [Folder]
    @Binding var selectedFolder: Folder?
    let stats: Stats?

    var body: some View {
        List {
            Section("Library") {
                Button(action: { selectedFolder = nil }) {
                    Label("All Media", systemImage: "photo.on.rectangle")
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .padding(.vertical, 2)
                .background(selectedFolder == nil ? Color.accentColor.opacity(0.2) : Color.clear)
                .cornerRadius(4)
            }

            if !folders.isEmpty {
                Section("Folders") {
                    ForEach(organizedFolders, id: \.path) { folder in
                        Button(action: { selectedFolder = folder }) {
                            FolderRow(folder: folder)
                                .contentShape(Rectangle())
                        }
                        .buttonStyle(.plain)
                        .padding(.vertical, 2)
                        .background(selectedFolder == folder ? Color.accentColor.opacity(0.2) : Color.clear)
                        .cornerRadius(4)
                    }
                }
            }

            if let stats = stats {
                Section("Stats") {
                    VStack(alignment: .leading, spacing: 4) {
                        StatRow(label: "Photos", value: "\(stats.totalPhotos)")
                        StatRow(label: "Videos", value: "\(stats.totalVideos)")
                        StatRow(label: "Folders", value: "\(stats.totalFolders)")
                        StatRow(label: "Total Size", value: formatSize(stats.totalOriginalMB))
                    }
                    .font(.caption)
                    .foregroundColor(.secondary)
                }
            }
        }
        .listStyle(.sidebar)
    }

    private var organizedFolders: [Folder] {
        folders.sorted { $0.path < $1.path }
    }

    private func formatSize(_ mb: Int64) -> String {
        if mb >= 1024 {
            return String(format: "%.1f GB", Double(mb) / 1024.0)
        }
        return "\(mb) MB"
    }
}

struct FolderRow: View {
    let folder: Folder

    var body: some View {
        HStack {
            Image(systemName: "folder")
                .foregroundColor(.accentColor)

            VStack(alignment: .leading) {
                Text(folder.displayName)
                    .lineLimit(1)

                if !folder.path.isEmpty && folder.path != folder.displayName {
                    Text(folder.path)
                        .font(.caption2)
                        .foregroundColor(.secondary)
                        .lineLimit(1)
                }
            }

            Spacer()

            Text("\(folder.photoCount)")
                .font(.caption)
                .foregroundColor(.secondary)
                .padding(.horizontal, 6)
                .padding(.vertical, 2)
                .background(Color.secondary.opacity(0.2))
                .cornerRadius(4)
        }
    }
}

struct StatRow: View {
    let label: String
    let value: String

    var body: some View {
        HStack {
            Text(label)
            Spacer()
            Text(value)
                .fontWeight(.medium)
        }
    }
}
