<%@ Page Title="" Language="C#" MasterPageFile="~/Site.Master" AutoEventWireup="true" CodeBehind="Proizvodi.aspx.cs" Inherits="WebApplication6.Account.Proizvodi" %>
<asp:Content ID="Content1" ContentPlaceHolderID="MainContent" runat="server">
    <asp:GridView ID="GridView1" runat="server" AutoGenerateColumns="False" DataSourceID="SqlDataSource2" DataKeyNames="Sifra">
        <Columns>
            <asp:BoundField DataField="Sifra" HeaderText="Sifra" ReadOnly="True" InsertVisible="False" SortExpression="Sifra"></asp:BoundField>
            <asp:BoundField DataField="Naziv" HeaderText="Naziv" SortExpression="Naziv"></asp:BoundField>
            <asp:BoundField DataField="Cena" HeaderText="Cena" SortExpression="Cena"></asp:BoundField>
            <asp:BoundField DataField="Akciska_cena" HeaderText="Akciska_cena" SortExpression="Akciska_cena"></asp:BoundField>
        </Columns>
    </asp:GridView>
    <asp:SqlDataSource runat="server" ID="SqlDataSource2" ConnectionString="<%$ ConnectionStrings:lazarConnectionString %>" SelectCommand="SELECT [Sifra], [Naziv], [Cena], [Akciska_cena] FROM [proizvodi]"></asp:SqlDataSource>
    <asp:SqlDataSource runat="server" ID="SqlDataSource1" ConnectionString="<%$ ConnectionStrings:lazarConnectionString %>" SelectCommand="SELECT [Naziv], [Cena], [Akciska_cena] FROM [proizvodi]"></asp:SqlDataSource>
    <asp:DropDownList runat="server" DataTextField="Sifra" DataValueField="Sifra" DataSourceID="SqlDataSource3" ID="ctl00"></asp:DropDownList><asp:SqlDataSource runat="server" ID="SqlDataSource3" ConnectionString="<%$ ConnectionStrings:lazarConnectionString %>" SelectCommand="SELECT [Sifra] FROM [proizvodi]"></asp:SqlDataSource>
</asp:Content>
