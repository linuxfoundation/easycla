# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

from hug.middleware import LogMiddleware
from datetime import datetime
from timeit import default_timer

class CLALogMiddleware(LogMiddleware):
    """CLA log middleware"""

    def __init__(self, logger=None):
        super().__init__(logger=logger)
        self.elapsed_time = 0
        self.start_time = None
        self.end_time = None

    def process_request(self, request, response):
        """Logs CLA request """
        self.logger.info(f'BEGIN {request.method} {request.path}')
        self.start_time = datetime.utcnow()
        super().process_request(request, response)
        
    def process_response(self, request, response, resource, req_succeeded):
        """Logs data returned by CLA API """
        if self.start_time:
            self.elapsed_time = datetime.utcnow() - self.start_time
        super().process_response(request, response, resource, req_succeeded)
        self.logger.info(f'END {request.method} {request.path} - elapsed_time : {self.elapsed_time.seconds} secs')

