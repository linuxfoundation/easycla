// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

import { Component } from '@angular/core';
import { environment } from 'src/environments/environment';
import { EnvConfig } from './config/cla-env-utils';
import { LfxHeaderService } from './core/services/lfx-header.service';

@Component({
  selector: 'app-root',
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.scss']
})
export class AppComponent {
  hasExpanded: boolean;

  constructor(
    private lfxHeaderService: LfxHeaderService
  ) {
    this.hasExpanded = true;

    this.mountHeader();
    this.mountFooter();
  }

  onToggled() {
    this.hasExpanded = !this.hasExpanded;
  }

  mountHeader() {
    const script = document.createElement('script');
    script.setAttribute('src', environment.lfxHeader);
    document.head.appendChild(script);
  }

  mountFooter() {
    const script = document.createElement('script');
    script.setAttribute(
      'src',
      EnvConfig.default['lfx-footer']
    );
    document.head.appendChild(script);
  }
}



