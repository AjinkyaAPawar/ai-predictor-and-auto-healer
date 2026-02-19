#!/usr/bin/env python3
"""
AI Predictor & Auto-Healer Demo App — Intentionally Problematic Service
This app simulates various infrastructure problems to showcase the healer's capabilities.
"""
import os
import time
import random
import socket
from http.server import HTTPServer, BaseHTTPRequestHandler

# Simulated problems (will trigger healer actions)
memory_leak = []
tmp_files = []
problem_mode = os.getenv("PROBLEM_MODE", "all")  # all, memory, disk, network, crash

class ProblemHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        """Handle GET requests and introduce problems"""
        
        # Memory leak — allocate and never free
        if problem_mode in ["all", "memory"]:
            # Grow by 10MB each request
            memory_leak.append(" " * (10 * 1024 * 1024))
        
        # Fill /tmp with junk files
        if problem_mode in ["all", "disk"]:
            try:
                filename = f"/tmp/junk_{random.randint(1,999999)}.tmp"
                with open(filename, 'w') as f:
                    f.write("X" * (5 * 1024 * 1024))  # 5MB files
                tmp_files.append(filename)
            except:
                pass
        
        # Simulate network/DNS issues periodically
        if problem_mode in ["all", "network"] and random.random() < 0.3:
            try:
                # Try to resolve a non-existent domain
                socket.gethostbyname("this-domain-does-not-exist-12345.invalid")
            except:
                pass
        
        # Random crashes to trigger restart analysis
        if problem_mode in ["all", "crash"] and random.random() < 0.05:
            self.send_response(500)
            self.end_headers()
            self.wfile.write(b"Simulated crash!")
            # Exit to trigger restart
            os._exit(137)  # Simulate OOMKill
        
        # Normal response
        self.send_response(200)
        self.send_header("Content-type", "text/html")
        self.end_headers()
        
        response = f"""
        <html>
        <head>
            <title>Demo App — Problematic Service</title>
            <style>
                body {{ 
                    font-family: monospace; 
                    background: #1a1a1a; 
                    color: #00ff00; 
                    padding: 40px;
                    text-align: center;
                }}
                h1 {{ color: #ff0055; }}
                .metric {{ 
                    background: #2a2a2a; 
                    padding: 20px; 
                    margin: 20px auto;
                    max-width: 600px;
                    border: 2px solid #00ff00;
                }}
            </style>
        </head>
        <body>
            <h1>⚠️ Problematic Demo App</h1>
            <p>This app is intentionally broken to demonstrate AI Healer capabilities.</p>
            
            <div class="metric">
                <h3>Memory Leak Status</h3>
                <p>Allocated: {len(memory_leak) * 10} MB</p>
                <p>Growing on every request...</p>
            </div>
            
            <div class="metric">
                <h3>/tmp Disk Usage</h3>
                <p>Junk files created: {len(tmp_files)}</p>
                <p>Space wasted: {len(tmp_files) * 5} MB</p>
            </div>
            
            <div class="metric">
                <h3>Active Problems</h3>
                <p>Mode: {problem_mode}</p>
                <p>🔥 Memory leak active</p>
                <p>🔥 Disk space filling</p>
                <p>🔥 Random DNS failures</p>
                <p>🔥 Random crashes (5% chance)</p>
            </div>
            
            <p><small>Refresh this page to trigger more issues...</small></p>
        </body>
        </html>
        """.format(
            len=len,
            problem_mode=problem_mode,
            memory_leak=memory_leak,
            tmp_files=tmp_files
        )
        
        self.wfile.write(response.encode())
    
    def log_message(self, format, *args):
        """Suppress request logging"""
        pass

def main():
    port = int(os.getenv("PORT", "8080"))
    server = HTTPServer(("0.0.0.0", port), ProblemHandler)
    
    print(f"🚨 Problematic Demo App starting on port {port}")
    print(f"   Problem mode: {problem_mode}")
    print(f"   This app will:")
    print(f"     - Leak memory continuously")
    print(f"     - Fill /tmp with junk files")
    print(f"     - Cause random DNS failures")
    print(f"     - Crash randomly (5% of requests)")
    print(f"   The AI Healer should detect and fix these issues!")
    
    server.serve_forever()

if __name__ == "__main__":
    main()
