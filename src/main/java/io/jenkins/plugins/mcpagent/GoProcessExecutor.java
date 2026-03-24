package io.jenkins.plugins.mcpagent;

import java.io.*;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermission;
import java.util.HashSet;
import java.util.Set;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * Executes the Go binary as a subprocess.
 * Handles OS detection, binary extraction from JAR, and process management.
 */
public class GoProcessExecutor {

    private static final Logger LOGGER = Logger.getLogger(GoProcessExecutor.class.getName());
    private static Path extractedBinaryPath;

    /**
     * Execute the Go binary with the given request JSON.
     * The Go binary runs asynchronously; this method returns immediately with the output.
     */
    public static String execute(String requestJson) throws IOException {
        Path binaryPath = getOrExtractBinary();

        ProcessBuilder pb = new ProcessBuilder(
                binaryPath.toString(),
                "analyze",
                "--request", requestJson,
                "--async", "true"
        );

        pb.redirectErrorStream(true);
        Process process = pb.start();

        // Read stdout (non-blocking, the Go binary prints the analysis ID and returns)
        StringBuilder output = new StringBuilder();
        try (BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()))) {
            String line;
            while ((line = reader.readLine()) != null) {
                output.append(line).append("\n");
            }
        }

        return output.toString().trim();
    }

    /**
     * Get the path to the extracted binary, extracting it if necessary.
     */
    private static synchronized Path getOrExtractBinary() throws IOException {
        if (extractedBinaryPath != null && Files.exists(extractedBinaryPath)) {
            return extractedBinaryPath;
        }

        String binaryName = getBinaryResourceName();
        String resourcePath = "/binaries/" + binaryName;

        InputStream binaryStream = GoProcessExecutor.class.getResourceAsStream(resourcePath);
        if (binaryStream == null) {
            throw new IOException("Go binary not found in plugin JAR: " + resourcePath);
        }

        // Extract to temp directory
        Path tempDir = Files.createTempDirectory("mcp-agent-");
        Path binaryPath = tempDir.resolve(binaryName);

        try (OutputStream out = Files.newOutputStream(binaryPath)) {
            byte[] buffer = new byte[8192];
            int bytesRead;
            while ((bytesRead = binaryStream.read(buffer)) != -1) {
                out.write(buffer, 0, bytesRead);
            }
        } finally {
            binaryStream.close();
        }

        // Make executable on Unix systems
        makeExecutable(binaryPath);

        extractedBinaryPath = binaryPath;
        LOGGER.log(Level.INFO, "Extracted Go binary to: {0}", binaryPath);

        // Register shutdown hook to clean up
        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            try {
                Files.deleteIfExists(binaryPath);
                Files.deleteIfExists(tempDir);
            } catch (IOException e) {
                LOGGER.log(Level.WARNING, "Failed to clean up temp binary", e);
            }
        }));

        return binaryPath;
    }

    /**
     * Determine the correct binary resource name based on OS and architecture.
     */
    private static String getBinaryResourceName() {
        String os = System.getProperty("os.name", "").toLowerCase();
        String arch = System.getProperty("os.arch", "").toLowerCase();

        String osName;
        if (os.contains("linux")) {
            osName = "linux";
        } else if (os.contains("mac") || os.contains("darwin")) {
            osName = "darwin";
        } else if (os.contains("win")) {
            osName = "windows";
        } else {
            osName = "linux"; // Default to Linux
        }

        String archName;
        if (arch.contains("aarch64") || arch.contains("arm64")) {
            archName = "arm64";
        } else {
            archName = "amd64";
        }

        String suffix = osName.equals("windows") ? ".exe" : "";
        return "mcp-agent-" + osName + "-" + archName + suffix;
    }

    /**
     * Make the binary executable on Unix-like systems.
     */
    private static void makeExecutable(Path path) {
        try {
            Set<PosixFilePermission> perms = new HashSet<>();
            perms.add(PosixFilePermission.OWNER_READ);
            perms.add(PosixFilePermission.OWNER_WRITE);
            perms.add(PosixFilePermission.OWNER_EXECUTE);
            perms.add(PosixFilePermission.GROUP_READ);
            perms.add(PosixFilePermission.GROUP_EXECUTE);
            Files.setPosixFilePermissions(path, perms);
        } catch (UnsupportedOperationException e) {
            // Windows doesn't support POSIX permissions; not needed there
        } catch (IOException e) {
            LOGGER.log(Level.WARNING, "Failed to set executable permission on binary", e);
        }
    }
}
