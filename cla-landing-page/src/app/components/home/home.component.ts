import {Component, OnInit} from '@angular/core';
import { environment } from 'src/environments/environment';
import {EnvConfig} from "../../config/cla-env-utils";

@Component({
  selector: 'app-home',
  templateUrl: './home.component.html',
  styleUrls: ['./home.component.scss']
})
export class HomeComponent implements OnInit {
  projectSection: any;
  organizationSection: any;
  developerSection: any;

  constructor() {
  }

  ngOnInit(): void {
    this.initializeData();
  }


  initializeData() {
    this.projectSection = {
      class: 'coding-icon',
      title: 'Projects',
      subtitle: 'Reduce administrative hassles of supporting the CLA for your project.',
      highlights: [
        'Look in one place to find the companies and individuals who have signed the CLA',
        'Enable Companies to manage authorization of their own developers',
        'Support both Individual and Corporate Contributors within a single portal'
      ],
      footerTitle: 'Are you a Community Manager or Project Manager?'
    };

    this.organizationSection = {
      class: 'building-icon',
      title: 'Organizations',
      subtitle: 'Enable all your developers to contribute code easily and quickly while remaining compliant with contribution policies.',
      highlights: [
        'Approved developers based on email, domain, GitHub handle, GitHub organization, GitLab handle, or GitLab Group',
        'Enable your signatories and contributors to sign CLAs using DocuSign® electronic signatures',
        'Enforce signing of the Corporate CLA by your developers without slowing them down with manual bureaucracy'
      ],
      footerTitle: 'Are you a CLA Manager for your organization?'
    };

    this.developerSection = {
      class: 'user-icon',
      title: 'Developers',
      subtitle: 'Get started contributing code faster and with less friction.',
      highlights: [
        'Receive an automatic notification in GitHub, GitLab or Gerrit if you need to be authorized',
        'Sign your Individual CLA with an e-signature',
        'Start contributing faster with a streamlined authorization workflow for Corporate CLAs'
      ],
      footerTitle: 'Do you contribute code to a project that uses CLAs?'
    };
  }

  mounted() {
    const script = document.createElement('script');
    script.setAttribute('src', environment.lfxHeader);
    document.head.appendChild(script);
  }
}
