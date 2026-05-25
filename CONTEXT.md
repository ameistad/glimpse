# Glimpse

Glimpse is a personal media library for quickly browsing a large home photo archive.

## Language

**Media Library**:
The complete collection of browsable media available through Glimpse.
_Avoid_: Archive, catalog

**Media Item**:
A single browsable photo or video in the **Media Library**.
_Avoid_: Asset, file, photo when referring to both photos and videos

**Original**:
The source media file kept in the user's photo archive.
_Avoid_: Raw, master, source file

**Thumbnail**:
A lightweight preview image derived from an **Original**.
_Avoid_: Preview, rendition

**Folder**:
A path-based grouping of **Media Items** within the **Media Library**.
_Avoid_: Album, collection

## Relationships

- A **Media Library** contains zero or more **Media Items**
- A **Media Item** belongs to exactly one **Folder**
- A **Media Item** has exactly one **Original**
- A **Media Item** may have one **Thumbnail**

## Example Dialogue

> **Dev:** "When someone opens a **Folder**, should they see every **Media Item** below it or only direct children?"
> **Domain expert:** "They should see everything below it, because folders represent path scopes in the **Media Library**, not manually curated albums."

## Flagged Ambiguities

- "photo" is used broadly in the codebase, but the domain includes both still images and videos. Use **Media Item** when the concept includes both.
